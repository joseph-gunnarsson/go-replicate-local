package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK on port " + port))
	})
	log.Printf("Auth service is starting on port %s", port)
	err := http.ListenAndServe(":"+port, mux)

	if err != nil {
		log.Fatalf("Auth service failed to start: %v", err)
	}

}
