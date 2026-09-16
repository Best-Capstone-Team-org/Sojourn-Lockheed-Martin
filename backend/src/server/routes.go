package server

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func Serve() {
	mux := routes()
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := fmt.Sprintf("0.0.0.0:%s", port)

	log.Printf("Listening and serving on %s... ", addr)

	err := http.ListenAndServe(addr, mux)

	if err != nil {
		log.Printf("ListenAndServe error: %+v", err)
	}
}

func routes() *http.ServeMux {
	mux := http.NewServeMux()

	fileServer := http.FileServer(http.Dir("../../frontend/build"))
	mux.Handle("/", fileServer)

	return mux
}
