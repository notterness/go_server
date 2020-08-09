/*
** This implements a basic HTTP server that responds to GET and POST verbs.
 */
package main

import (
	"context"
	"log"
	"net/http"
	"sync"
)

/*
** The following sync httpShutdownRequested is triggered when the /shutdown request is received and there are
**   no outstanding requests being processed.
 */
var httpShutdownRequested sync.WaitGroup

func main() {
	log.Printf("main: starting HTTP server")

	// The httpServerExitDone WaitGroup is used to inform main() that the server has successfully exited and the
	//   program is now ready to finish shutting down.
	httpServerExitDone := &sync.WaitGroup{}
	httpServerExitDone.Add(1)

	// The httpShutdownRequested is set when the curl request for "/shutdown" is made and the program can start
	//   waiting for the outstanding requests to drain. While the requests are draining, any new requests will
	//   be responded to by the JSON object that returns {"error": 503}. 503 was chosen as it means:
	//   SERVICE_UNAVAILABLE_503
	httpShutdownRequested.Add(1)

	srv := startHttpServer(httpServerExitDone)

	// now close the server gracefully ("shutdown")
	httpShutdownRequested.Wait()
	if err := srv.Shutdown(context.TODO()); err != nil {
		panic(err) // failure/timeout shutting down the server gracefully
	}

	// wait for goroutine started in startHttpServer() to stop
	httpServerExitDone.Wait()

	log.Printf("main: exiting")
}

/*
** THis starts up the actual HTTP server which is listening on port 8080.
**
** The server is setup with only a single handler function that all requests are routed through. This is done to
**   simplify the handling of the shutdown process and to provide the ability to have different handlers
**   that are used depending upon the state of the server. In this case, the states are simple, either running
**   or in the process of being shut down.
 */
func startHttpServer(wg *sync.WaitGroup) *http.Server {

	// Setup the initial HTTP Request handler map. This set of handlers covers the following methods:
	//   POST /hash
	//   GET /stats
	//   PUT, POST, GET /shutdown
	initialize()

	// Start the HTTP Server running on port 8080
	srv := &http.Server{Addr: ":8080"}

	// All HTTP requests go through the common handler and then the URL is parsed to determine which
	//   actual handler to use. This is done to allow the handlers to be changed on the fly once the
	//   /shutdown method is processed.
	http.HandleFunc("/", handler) // each request calls handler

	go func() {
		defer wg.Done() // let main know we are done cleaning up

		// always returns error. ErrServerClosed on graceful close
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			// unexpected error. port in use?
			log.Fatalf("ListenAndServe(): %v", err)
		}
	}()

	// returning reference so caller can call Shutdown()
	return srv
}
