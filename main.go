package main

import (
	"net/http"
)


func main() {
	mux := http.NewServeMux()
	server := http.Server{}
	server.Addr = ":8080"
	server.Handler = mux

	server.ListenAndServe()
}
