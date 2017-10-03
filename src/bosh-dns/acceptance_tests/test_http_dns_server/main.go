package main

import (
	"fmt"
	"net"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(responseWriter http.ResponseWriter, req *http.Request) {

		if req.URL.RawQuery != "type=1&name=app-id.internal.local." {
			panic(fmt.Sprintf("Expected query 'type=1&name=app-id.internal.local.' Actual: '%s'", req.URL.RawQuery))
		}

		responseJSON := `{
					"Status": 0,
					"TC": false,
					"RD": true,
					"RA": true,
					"AD": false,
					"CD": false,
					"Question":
					[
						{
							"name": "app-id.internal.local.",
							"type": 1
						}
					],
					"Answer":
					[
						{
							"name": "app-id.internal.local.",
							"type": 1,
							"TTL":  0,
							"data": "192.168.0.1"
						}
					],
					"Additional": [ ],
					"edns_client_subnet": "12.34.56.78/0"
				}`

		responseWriter.Write([]byte(responseJSON))
	})

	listener, err := net.Listen("tcp", "0.0.0.0:8081")
	if err != nil {
		panic(err)
	}

	err = http.Serve(listener, nil)
	if err != nil {
		panic(err)
	}
}
