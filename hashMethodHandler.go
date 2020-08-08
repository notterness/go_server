package main

import (
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var mu sync.Mutex
var count = 0

/*
** The requiredFormFields array of String is used to validate form data that is passed into the "POST /hash"
**   method. Currently, there is only one required form field, but to add more, simply update the
**   RequiredFormFields constant and add the values in the initializeHash() function.
 */
const RequiredFormFields = 1
const PasswordFormField = "password"

var requiredFormFields [RequiredFormFields]string

/*
** Setup the required form fields. This uses an array to make the addition of additional required form fields easy.
 */
func initializeHash() {
	requiredFormFields[0] = PasswordFormField
}

/*
** This is the handler for the "PUT /hash" method. If there is not an error in the parsing of either the
**   method fields or the form data, it will return the number of times this has been called (inclusive of ths call).
 */
func hash(w http.ResponseWriter, r *http.Request) {

	defer measurePostTime(time.Now().UnixNano())

	/*
	** Duplicate code, but rather than passing in a different parameter (and making the method handler maps way more
	**   complicated) re-parse the URL and see if there is only the "hash" filed (known to be true if the code got here)
	**   or if there is a endpoint identifier that follows the /hash/<new field>
	 */
	methodStrings := strings.Split(r.URL.RequestURI(), "/")
	for i := range methodStrings {
		fmt.Printf("hash() index %d - %s\n", i, methodStrings[i])
	}
	fmt.Printf("hash() number strings: %d\n", len(methodStrings))

	/*
	** Parse out the form fields and make sure that "password" is present
	 */
	if err := r.ParseForm(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "hashWithQualifier() ParseForm: %v\n", err)
	}
	for k, v := range r.Form {
		fmt.Fprintf(w, "Form[%q] = %q\n", k, v)
	}

	if validateFormData(r) {
		numOfStr := len(methodStrings)
		if numOfStr == 2 {
			mu.Lock()
			count++
			tmp := count
			mu.Unlock()

			n, err := fmt.Fprintf(w, "%d\n", tmp)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "hash(1) Fprintf: %d %v\n", n, err)
			}
		} else {
			/*
			** UNPROCESSABLE_ENTITY_422
			**
			** Since the number of qualifiers was not 0, return UNPROCESSABLE_ENTITY since the code should not
			**   return anything unexpected method qualifiers.
			 */
			n, err := fmt.Fprintf(w, "{\"error\": 422}\n")
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "hash(2) Fprintf: %d %v\n", n, err)
			}
		}
	} else {
		/*
		** PRECONDITION_FAILED_412
		**
		** If all of the required form fields are not present, return the PRECONDITION_FAILED error code
		 */
		n, err := fmt.Fprintf(w, "{\"error\": 412}\n")
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "hash(3) Fprintf: %d %v\n", n, err)
		}
	}
}

/*
** This is the hash function that is called from the GET verb
 */
func hashWithQualifier(w http.ResponseWriter, r *http.Request) {

	/*
	** Duplicate code, but rather than passing in a different parameter (and making the method handler maps way more
	**   complicated) re-parse the URL and see if there is only the "hash" filed (known to be true if the code got here)
	**   or if there is a endpoint identifier that follows the /hash/<new field>
	 */
	methodStrings := strings.Split(r.URL.RequestURI(), "/")
	for i := range methodStrings {
		fmt.Printf("hash() index %d - %s\n", i, methodStrings[i])
	}
	fmt.Printf("hash() number strings: %d\n", len(methodStrings))

	numOfStr := len(methodStrings)
	if numOfStr == 3 {
		/*
		** Validate that the field is an integer
		 */
		i, err := strconv.ParseInt(methodStrings[2], 10, 32)
		if err == nil {
			performHash(i, "angryMonkey")
		} else {
			/*
			** UNPROCESSABLE_ENTITY_422
			**
			** Since the value passed in was not an integer, return UNPROCESSABLE_ENTITY since the code should not
			**   return anything for a garbage method qualifier.
			 */
			n, err := fmt.Fprintf(w, "{\"error\": 422}\n")
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "hashWithQualifier(1) Fprintf: %d %v\n", n, err)
			}
		}
	} else {
		/*
		** UNPROCESSABLE_ENTITY_422
		**
		** Since the number of qualifiers was not 1, return UNPROCESSABLE_ENTITY since the code should not
		**   return anything unexpected method qualifiers.
		 */
		n, err := fmt.Fprintf(w, "{\"error\": 422}\n")
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "hashWithQualifier(2) Fprintf: %d %v\n", n, err)
		}
	}
}

func performHash(qualifier int64, password string) {
	h := sha512.New()
	h.Write([]byte(password))
	base64ResultStr := base64.StdEncoding.EncodeToString(h.Sum(nil))

	n, err := fmt.Printf("%d base64: %s", qualifier, base64ResultStr)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Fprintf: %d %v\n", n, err)
	}
}

/*
** This function is used to validate the form data that is passed in from the client. It insures that the
**   required form fields are present.
 */
func validateFormData(r *http.Request) bool {
	var success = true

	for i := 0; i < RequiredFormFields; i++ {
		result := r.FormValue(requiredFormFields[i])
		if len(result) == 0 {
			success = false
		}
	}

	return success
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
	log.Printf("POST /hash took %d", elapsed)

	mu.Lock()
	totalTime += elapsed
	mu.Unlock()
}
