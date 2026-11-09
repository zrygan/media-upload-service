package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

// this'll be a Go HTTP server that looks at the media/data folder where videos are dumped and can
// send them back to the web app when needed

func mainHandler(writer http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		http.NotFound(writer, req)
		return
	}

	fmt.Fprint(writer, "hello world :3")
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", mainHandler)

	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		WriteTimeout: 20 * time.Second,
		ReadTimeout:  20 * time.Second,
		IdleTimeout:  80 * time.Second,
	}

	log.Printf("starting server on port %s\n", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed to start: %v", err)
	}
}
