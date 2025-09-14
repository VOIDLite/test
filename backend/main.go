package main

import (
	"log"

	"backend/server"
)

func main() {
	log.Println("Starting server on :8080")
	srv := server.New("8080")
	log.Fatal(srv.ListenAndServe())
}
