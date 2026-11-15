package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The GUI's HTTP server: serves the web page, lists the videos in
// data/media, and streams them to the browser.

type VideoInfo struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

func mainHandler(writer http.ResponseWriter, req *http.Request) {
	writer.Header().Set("Content-Type", "application/json")

	videoDir := "./data/media"
	files, err := os.ReadDir(videoDir)
	if err != nil {
		http.Error(writer, "unable to read video directory", http.StatusInternalServerError)
		return
	}

	videos := []VideoInfo{}
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".mp4" {
			name := file.Name()
			title := name
			if i := strings.Index(name, "_"); i >= 0 {
				title = name[i+1:]
			}
			videos = append(videos, VideoInfo{
				Title: title,
				URL:   "/data/media/" + name,
			})
		}
	}

	json.NewEncoder(writer).Encode(videos)
}

func main() {
	mux := http.NewServeMux()

	fileServer := http.FileServer(http.Dir("./consumer/webapp"))
	mux.Handle("/webapp/", http.StripPrefix("/webapp/", fileServer))

	dataVideoServer := http.FileServer(http.Dir("./data/media"))
	mux.Handle("/data/media/", http.StripPrefix("/data/media/", dataVideoServer))

	mux.HandleFunc("/", func(writer http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/" {
			http.NotFound(writer, req)
			return
		}
		http.ServeFile(writer, req, "./consumer/webapp/index.html")
	})

	mux.HandleFunc("/api/videos/", mainHandler)

	server := &http.Server{
		Addr:         ":8008",
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
