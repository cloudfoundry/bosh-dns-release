package main

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
)

func main() {
	http.HandleFunc("/", func(responseWriter http.ResponseWriter, req *http.Request) {
		var responseJSON string
		queryValues, err := url.ParseQuery(req.URL.RawQuery)

		if err != nil {
			panic(fmt.Sprintf("Unexpected query '%s'", req.URL.RawQuery))
		}

		if queryValues.Get("type") == "1" && queryValues.Get("name") == "app-id.internal.local." {
			responseJSON = `{
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
		} else if queryValues.Get("type") == "1" && queryValues.Get("name") == "large-id.internal.local." {
			responseJSON = `{
					"Status": 0,
					"TC": false,
					"RD": true,
					"RA": true,
					"AD": false,
					"CD": false,
					"Question":
					[
						{
							"name": "large-id.internal.local.",
							"type": 1
						}
					],
					"Answer":
					[
						{
							"name": "large-id.internal-domain.",
							"type": 1,
							"TTL": 1526,
							"data": "192.168.0.1"
						},
						{
							"name": "large-id.internal-domain.",
							"type": 1,
							"TTL": 1526,
							"data": "192.168.0.2"
						},
						{
							"name": "large-id.internal-domain.",
							"type": 1,
							"TTL": 1526,
							"data": "192.168.0.3"
						},
						{
							"name": "large-id.internal-domain.",
							"type": 1,
							"TTL": 224,
							"data": "192.168.0.4"
						},
						{
							"name": "large-id.internal-domain.",
							"type": 1,
							"TTL": 224,
							"data": "192.168.0.5"
						},
						{
							"name": "large-id.internal-domain.",
							"type": 1,
							"TTL": 224,
							"data": "192.168.0.6"
						},
						{
							"name": "large-id.internal-domain.",
							"type": 1,
							"TTL": 224,
							"data": "192.168.0.7"
						},
						{
							"name": "large-id.internal-domain.",
							"type": 1,
							"TTL": 224,
							"data": "192.168.0.8"
						},
						{
							"name": "large-id.internal-domain.",
							"type": 1,
							"TTL": 224,
							"data": "192.168.0.9"
						},
						{
							"name": "large-id.internal-domain.",
							"type": 1,
							"TTL": 224,
							"data": "192.168.0.10"
						},
						{
							"name": "large-id.internal-domain.",
							"type": 1,
							"TTL": 224,
							"data": "192.168.0.11"
						},
						{
							"name": "large-id.internal-domain.",
							"type": 1,
							"TTL": 224,
							"data": "192.168.0.12"
						},
						{
							"name": "large-id.internal-domain.",
							"type": 1,
							"TTL": 224,
							"data": "192.168.0.13"
						}
					],
					"Additional": [ ],
					"edns_client_subnet": "12.34.56.78/0"
				}`
		} else {
			panic(fmt.Sprintf("Unexpected query '%s'", req.URL.RawQuery))
		}

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
