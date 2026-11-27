package main

import (
	"log"
	"net/http"
	handler "velveta-backend/api"
)

func main() {
	log.Println("Server lokal jalan di http://localhost:3000")
	log.Fatal(http.ListenAndServe(":3000", http.HandlerFunc(handler.Handler)))
}