package main

import (
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

var mu sync.Mutex
var count int = 1

const RequiredFormFields = 1
const PasswordFormField = "password"

var requiredFormFields [RequiredFormFields]string

/*
** Setup the required form fields. This uses an array to make the addition of additional required form fields easy.
 */
func initializeHash() {
	requiredFormFields[0] = PasswordFormField
}

func hash(w http.ResponseWriter, r *http.Request) {

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
		log.Print(err)
	}
	for k, v := range r.Form {
		fmt.Fprintf(w, "Form[%q] = %q\n", k, v)
	}

	if validateFormData(r) {
		numOfStr := len(methodStrings)
		if numOfStr == 2 {
			mu.Lock()
			tmp := count
			count++
			mu.Unlock()

			fmt.Fprintf(w, "Count %d\n", tmp)
		} else {
			/*
			** UNPROCESSABLE_ENTITY_422
			**
			** Since the number of qualifiers was not 0, return UNPROCESSABLE_ENTITY since the code should not
			**   return anything unexpected method qualifiers.
			 */
			fmt.Fprintf(w, "{\"error\": 422}\n")
		}
	} else {
		/*
		** PRECONDITION_FAILED_412
		**
		** If all of the required form fields are not present, return the PRECONDITION_FAILED error code
		 */
		fmt.Fprintf(w, "{\"error\": 412}\n")
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
			fmt.Fprintf(w, "{\"error\": 422}\n")
		}
	} else {
		/*
		** UNPROCESSABLE_ENTITY_422
		**
		** Since the number of qualifiers was not 0, return UNPROCESSABLE_ENTITY since the code should not
		**   return anything unexpected method qualifiers.
		 */
		fmt.Fprintf(w, "{\"error\": 422}\n")
	}
}

func performHash(qualifier int64, password string) {
	h := sha512.New()
	h.Write([]byte(password))
	base64ResultStr := string(base64.StdEncoding.EncodeToString(h.Sum(nil)))

	fmt.Printf("%d base64: %s", qualifier, base64ResultStr)
}

/*
** This function is used to validate the form data that is passed in from the client. It insures that the
**   required form fields are present.
 */
func validateFormData(r *http.Request) bool {
	var success bool = true

	for i := 0; i < RequiredFormFields; i++ {
		result := r.FormValue(requiredFormFields[i])
		if len(result) == 0 {
			success = false
		}
	}

	return success
}
