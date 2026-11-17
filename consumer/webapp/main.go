package main

import (
	"embed"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// The GUI's HTTP server: serves the web page, lists the videos in the
// media folder, and streams them to the browser. It runs as its own
// program next to the consumer and only reads the media folder.

// The page is built into the binary, so the GUI runs from any folder.
//
//go:embed index.html index.js
var static embed.FS

// VideoInfo is one entry in the list the page shows.
type VideoInfo struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

// listVideos returns every file in dir, oldest first. Workers only
// rename finished videos into dir, so every file there is complete.
// Hidden files are skipped.
func listVideos(dir string) ([]VideoInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	type file struct {
		name string
		mod  time.Time
	}
	var files []file
	for _, e := range entries {
		if !e.Type().IsRegular() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue // removed since ReadDir
		}
		files = append(files, file{e.Name(), info.ModTime()})
	}
	sort.SliceStable(files, func(i, j int) bool { return files[i].mod.Before(files[j].mod) })

	videos := make([]VideoInfo, 0, len(files))
	for _, f := range files {
		videos = append(videos, VideoInfo{
			Title: strings.TrimSuffix(f.name, filepath.Ext(f.name)),
			// escaped, so names with spaces, "#" or "?" still work
			URL: "/data/media/" + url.PathEscape(f.name),
		})
	}
	return videos, nil
}

func main() {
	addr := flag.String("addr", ":8008", "host:port to serve the GUI on")
	media := flag.String("media", "./data/media", "the consumer's media folder")
	flag.Parse()

	mux := http.NewServeMux()

	mux.Handle("/webapp/", http.StripPrefix("/webapp/", http.FileServerFS(static)))
	mux.Handle("/data/media/", http.StripPrefix("/data/media/", http.FileServer(http.Dir(*media))))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFileFS(w, r, static, "index.html")
	})

	mux.HandleFunc("/api/videos/", func(w http.ResponseWriter, r *http.Request) {
		videos, err := listVideos(*media)
		if err != nil {
			http.Error(w, "unable to read video directory", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(videos)
	})

	server := &http.Server{
		Addr:        *addr,
		Handler:     mux,
		ReadTimeout: 20 * time.Second,
		IdleTimeout: 80 * time.Second,
		// no WriteTimeout: it would cut off a long video while it plays
	}

	log.Printf("GUI on %s, media folder %s", server.Addr, *media)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed to start: %v", err)
	}
}
