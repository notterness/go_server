# go_server

To build the go_server executable, from the directory where the *.go files are, run:

go build

This will create an executable called go_server in that directory. To run the executable, simply use the following on the command line:

./go_server

At this point, the tests in go_server_test can be run against the server or curl commands can be run against the server.

The server can be accessed through "http://localhost/8080" (listening on port 8080).

The curl format for the POST /hash command is: curl -X POST -d "password"="angryMonkey" http://localhost/hash
This will return an <identifier> (the <identifier> is an interger) that can be used to retrieve the hashed password after 5 seconds.

The curl format for the GET /hash command to retrieve the hashed password is: curl http://localhost:8080/hash/<identifier which is an integer>
This will return the hashed password if it is issued at least 5 seconds after the POST /hash the returned the specified identifier.
If the request is made and the identifier is invalid (i.e. the POST /hash has only returned up to 5 and the GET /hash/6 is issued) the respomse
  will be 404 (NOT_FOUND).
If the request is made sooner than 5 seconds after the POST and the identifier is valid, the respose will still be 404 as the hash has not yet been computed.
