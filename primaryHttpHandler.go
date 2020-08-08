package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

/*
** The requestsMutex is used to proterct access to the outstandingRequests and shutdownRequested variables.
**   The problem that requires the mutex is the behavior of the outstandingRequests is dependent upon the
**   state of the shutdownRequested boolean, so the code is clearer if they are treated as an atomic unit rather than
**   two different atomic variables.
**
** NOTE: The outstandingRequests only keeps track of the number of requests that are in flight prior to the
**   shutdownRequested flag being set. Once the flag is set, all new requests are returned with the
**   SERVICE_UNAVAILABLE error while the number of outstandingRequests counts down to zero.
 */
var requestsMutex sync.Mutex

var outstandingRequests int32 = 0
var shutdownRequested bool = false

// There are three separate maps to handle the three different HTTP verbs that are supported.
//   POST /hash
//   POST /hash/<integer value>
//   GET /stats
//   generic /shutdown
var postHandlerMap map[string]func(http.ResponseWriter, *http.Request) = make(map[string]func(http.ResponseWriter, *http.Request))
var getHandlerMap map[string]func(http.ResponseWriter, *http.Request) = make(map[string]func(http.ResponseWriter, *http.Request))
var genericHandlerMap map[string]func(http.ResponseWriter, *http.Request) = make(map[string]func(http.ResponseWriter, *http.Request))

// There is one map to figure out which verbs are supported and which method map to use
var verbHttpMap map[string]map[string]func(http.ResponseWriter, *http.Request) = make(map[string]map[string]func(http.ResponseWriter, *http.Request))

/*
** The following are the supported methods
 */
const HashMethod = "hash"
const ShutdownMethod = "shutdown"
const StatsMethod = "stats"

/*
** The following are the supported HTTP verbs.
**
** NOTE: This current implementation does not support DELETE, PATCH and PUT
 */
const HttpGetVerb = "GET"
const HttpPostVerb = "POST"

/*
** This is used to setup the different maps used to determine which handler to execute based upon the HTTP verb and
**   the method.
 */
func initialize() {
	/*
	** First initialize anything the different method handlers required
	 */
	initializeHash()

	/*
	** Setup the handlers for the various HTTP verbs
	 */
	postHandlerMap[HashMethod] = hash
	postHandlerMap[ShutdownMethod] = shutdown
	postHandlerMap[""] = unsupportedRequest

	getHandlerMap[HashMethod] = hashWithQualifier
	getHandlerMap[StatsMethod] = stats
	getHandlerMap[ShutdownMethod] = shutdown

	// The generic map is used to handle requests that don't care what the verb type is (GET, PUT, POST)
	//   NOTE: For this implementation PUT, PATCH and DELETE are not supported and will return METHOD_NOT_ALLOWED_405
	genericHandlerMap[ShutdownMethod] = shutdown

	verbHttpMap[HttpGetVerb] = getHandlerMap
	verbHttpMap[HttpPostVerb] = postHandlerMap
	verbHttpMap["generic"] = genericHandlerMap

	fmt.Println(postHandlerMap)
}

/*
** There is a single HTTP request handler and this performs the dispatch to different sub-handlers based upon
**   the HTTP verb and the method. The simplest method is just to register the handlers directly with the
**   http server via the HandleFunc() call, but (maybe there is a better solution to this) the problem is that
**   the handlers need to change once the shutdown has been requested. The other solution would be to add code
**   in every handler to check for the shutdown state, but it makes more sense to consolidate it to a single
**   location.
** The single primary handler with a dispatch within it also make tracking the time spent easier as there is a single
**   place in which to measure the elapsed time instead of having duplicate code within each handler.
**
** This function uses multiple maps to determine which handler function to actually call. This allows for additional
**   handlers to be added quite easily and to differentiate between handlers for different HTTP verbs. For example,
**   the "/hash" handler is different for the POST and the GET verbs so registering a single handler in the
**   http.HandleFunc() would mean the differentiation in behavior would take place within the handler (a subtle
**   difference but one that makes the behavior a bit easier to to track).
** The first check is to determine which map of handlers to use based upon the HTTP verb. Once that is done, then the
**   code checks for the method (essentially split the string using the '/' token). The string following the first '/'
**   is used to search the map for the appropriate handler.
**
** NOTE: If the HTTP server needs to handle the case of an HTTP verb with an empty method (i.e. something
**   like "GET / HTTP/1.1") the checking of the map will need to be use an empty string for the search string.
 */
func handler(w http.ResponseWriter, r *http.Request) {

	fmt.Fprintf(w, "%s %s %s\n", r.Method, r.URL, r.Proto)

	shuttingDown := incOutstandingAndCheckForShutdown()
	if !shuttingDown {
		// Parse the URL to see if anything needs to be processed
		methodStrings := strings.Split(r.URL.RequestURI(), "/")
		for i := range methodStrings {
			fmt.Printf("index %d - %s\n", i, methodStrings[i])
		}
		fmt.Printf("%s number strings: %d\n", r.URL.RequestURI(), len(methodStrings))

		/*
		** See if there is an appropriate handler for passed in URL (only interested in the first entry to map the
		**   handler, but note that the strings.Split() call has a slightly odd behavior).
		** string.Split() results:
		**   input string "/" -> results in two methodStrings, each which are ""
		**   input string "/hash" -> results in two method strings, [0] is "". [1] is "hash"
		**   input string "/hash/1" -> results in three method strings, [0] is "", [1] is "hash", [3] is "1"
		** Since the only URL strings that need to be handled, insure that there is at least two parsed out
		**   method strings (due to the odd behavior of Split()).
		 */
		if len(methodStrings) >= 2 {
			var handlerMap map[string]func(http.ResponseWriter, *http.Request)

			/*
			** Only measure the time for POST /hash requests if the server is not shutting down
			 */
			if r.Method == HttpPostVerb && methodStrings[1] == HashMethod {
				defer measurePostTime(time.Now().UnixNano())
			}

			handlerMap = verbHttpMap[r.Method]
			if handlerMap != nil {
				fmt.Printf("Map lookup - %s\n", methodStrings[1])
				httpHandler := handlerMap[methodStrings[1]]
				if httpHandler != nil {
					httpHandler(w, r)
				} else {
					unsupportedRequest(w, r)
				}
			} else {
				verbNotSupported(w, r)
			}
		} else {
			unsupportedRequest(w, r)
		}

		decOutstandingAndCheckForShutdown()
	} else {
		/*
		** This is the code path when the shutdownRequested flag is set and the server is waiting for the
		**   outstanding requests to drain prior to performing the actual shutdown.
		 */
		failRequest(w, r)
	}
}

/*
** This function does two things:
**   First it checks if the "shutdownRequested" flag is set indicating that the client has called the server
**     with the /shutdown method. If that is true, then the function WILL NOT increment the "outstandingRequests"
**     count and will return "true" to indicate that the SERVICE_UNAVAILABLE_503 response should be sent.
**  Second, if the "shutdownRequested" flag is false, then it increments the "outstandingRequests" count and will return
**     "false" to indicate normal handling of requests should take place.
 */
func incOutstandingAndCheckForShutdown() bool {
	var shuttingDown bool = false

	requestsMutex.Lock()
	if shutdownRequested {
		shuttingDown = true
	} else {
		outstandingRequests++
	}
	requestsMutex.Unlock()

	return shuttingDown
}

/*
** This function will decrement the number of "outstandingRequests".
**   Once the number of "outstandingRequests" is decremented, it will check if a shutdown has been requested and if
**   the number of outstanding requests is 0, it will then signal the main() to trigger the shutdown of the HTTP server.
 */
func decOutstandingAndCheckForShutdown() {
	requestsMutex.Lock()
	outstandingRequests--
	if shutdownRequested && (outstandingRequests == 0) {
		httpShutdownRequested.Done()
	}
	requestsMutex.Unlock()
}

func stats(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	tmp := count
	var avg int = 0
	mu.Unlock()

	fmt.Fprintf(w, "{\"total\": %d, \"average\": %d}\n", tmp, avg)
}

/*
** The shutdown() handler is pretty simple in that is just sets a flag that is checked whenever a new
**   request comes in. The
 */
func shutdown(w http.ResponseWriter, r *http.Request) {
	requestsMutex.Lock()
	shutdownRequested = true

	/*
	** Need to handle the case where there are no requests currently outstanding and the shutdown can happen
	**   immediately.
	 */
	if outstandingRequests == 0 {
		httpShutdownRequested.Done()
	}

	requestsMutex.Unlock()
}

/*
** The following handler is used while the number of outstandingRequests ic counting down and a new request has been
**   received (this is after the shutdownRequested flag has been set). It tells the client the service is
**   no longer available.
 */
func failRequest(w http.ResponseWriter, r *http.Request) {
	// SERVICE_UNAVAILABLE_503
	fmt.Fprintf(w, "{\"error\": 503}\n")
}

/*
** This is used when the HTTP verb is supported, but the method the client requested is not supported
**   by the server. It returns a simple error of METHOD_NOT_ALLOWED_405 to the client.
 */
func unsupportedRequest(w http.ResponseWriter, r *http.Request) {
	// METHOD_NOT_ALLOWED_405
	fmt.Printf("unsupportedRequest\n")
	fmt.Fprintf(w, "{\"error\": 405}\n")
}

/*
** This function is called when the HTTP verb passed into the top level handler method does not match any of the
**   supported verbs.
** This returns the METHOD_NOT_ALLOWED_405 and the list of supported HTTP verbs.
 */
func verbNotSupported(w http.ResponseWriter, r *http.Request) {
	// METHOD_NOT_ALLOWED_405
	fmt.Fprintf(w, "{\n   {\"error\": 405},\n   {\"Allow\": GET PUT POST}\n}\n")
}

/*
** The following is a deferred function used to compute the time required to handle the POST functions.
**
** NOTE: This is not a perfect representation of the processing time for the POST /hash handler as there is
**   work that was done prior to this call to determine the request type (and the work in the HTTP server just to
**   receive the request). But, it does give a measure of the specific handler so that can be evaluated for
**   improvements.
 */
func measurePostTime(start int64) {
	elapsed := (time.Now().UnixNano() - start) / int64(time.Microsecond)
	log.Printf("POST /hash took %s", elapsed)
}
