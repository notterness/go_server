# go_server

To build the go_server executable, from the directory where the *.go files are, run:

go build

This will create an executable called go_server in that directory. To run the executable, simply use the following on the command line:

./go_server

At this point, the tests in go_server_test can be run against the server or curl commands can be run against the server.

The server can be accessed through "http://localhost/8080" (listening on port 8080).

The curl format for the POST /hash request is: curl -X POST -d "password"="angryMonkey" http://localhost:8080/hash
This will return an "identifier" (the "identifier" is an integer) that can be used to retrieve the hashed password after 5 seconds.

The curl format for the GET /hash request to retrieve the hashed password is: curl http://localhost:8080/hash/"identifier" which is an integer>
This will return the hashed password if it is issued at least 5 seconds after the POST /hash the returned the specified "identifier".
If the request is made and the "identifier" is invalid (i.e. the POST /hash has only returned up to 5 and the GET /hash/6 is issued) the respomse
  will be 404 (NOT_FOUND).
If the request is made sooner than 5 seconds after the POST and the identifier is valid, the respose will still be 404 as the hash has not yet been computed.

The curl format for the GET /stats request to retrieve the statistics values is: curl http://localhost:8080/stats.

The curl format for the /shutdown request is either: curl http://localhost:8080/shutdown or curl -X POST http://localhost:8080/hash/

The go_server has the following behavior:

1) The POST /hash requests can all provide different passwords and the password hash is tied to the value that is the response. I refer to that value as the "identifier"
   as they are all unique to the POST /hash request. The hashed password that is returned from the GET /hash/"identifier" is the one (assuming 5 seconds have gone by)
   that was sent to the POST /hash request that returned the "identifier". So, it is possible to have 42 different hash values for 42 POST /hash requests.

2) For the GET /hash/"identifier" I have it return immediately regardless if the hash has been computed or not. If the hash has not been computed (meaning it was either a
   bad "identifier" or the POST /hash for the "identifier" was not five seconds in the past) it will return a NOT_FOUND_404 error. If the hash has been computed, it will
   return the specified hash of the password. The immediate return of the GET /hash/"identifier" allows the client to poll until the hash is available.

3) The go_server listens on port 8080.

4) The only supported HTTP verbs are GET and POST. Any other verb will return with a METHOD_NOT_ALLOWED_405 response and the list of allowed verbs.

5) Methods under the GET and POST verbs that are not supported will return a METHOD_NOT_ALLOWED_405 response.

6) For the POST /hash method if the form data does not contain a "password" entry, it will return a PRECONDITION_FAILED_412 error.

7) For the GET /hash/"identifier" request, if there is not an "identifier" or the "identifier" is not an integer it will return a UNPROCESSABLE_ENTITY_422 error.

8) The /shutdown method is supported for both the GET and PUT HTTP verbs.

9) While the /shutdown method is waiting for outstanding requests to complete, the server will respond with the SERVICE_UNAVAILABLE_503 error to all new requests.
 

