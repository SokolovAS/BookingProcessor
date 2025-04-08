package main

import (
	"log"
	"net/http"
	_ "net/http/pprof" // registers pprof handlers on the default mux
)

func main() {
	log.Println("Starting debug pprof server on port 6060")
	if err := http.ListenAndServe(":6060", nil); err != nil {
		log.Fatalf("Error starting pprof server: %v", err)
	}
}
