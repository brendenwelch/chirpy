package main

import (
	"log"
	"net/http"
)

func main() {
	var server http.Server
	server.Handler = http.NewServeMux()
	server.Addr = ":8080"
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
