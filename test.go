package main

import (
	"log"
	"net/http"
)

func main() {
	log.Println("SERVER STARTING...")
	http.ListenAndServe(":8080", nil)
}
