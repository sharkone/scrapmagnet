package main

import (
	"fmt"
	"github.com/drone/routes"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
)

type Server struct {
	settings   *Settings
	downloader *Downloader
	magnets    map[string]*Magnet
}

func NewServer(settings *Settings) *Server {
	return &Server{settings: settings, downloader: NewDownloader(settings), magnets: make(map[string]*Magnet)}
}

func (server *Server) Run() {
	server.downloader.Start()
	log.Println("[HTTP] Listening on port", server.settings.http.port)

	mux := routes.New()
	mux.Get("/add", add)
	mux.Get("/files", files)
	mux.Get("/files/:infohash", files)
	mux.Get("/files/:infohash/:file", files)
	mux.Get("/shutdown", shutdown)

	http.Handle("/", mux)
	http.ListenAndServe(":"+strconv.Itoa(server.settings.http.port), nil)
}

func add(w http.ResponseWriter, r *http.Request) {
	magnetLink := r.URL.Query().Get("magnet")

	downloadDir := r.URL.Query().Get("download_dir")
	if downloadDir == "" {
		downloadDir = "."
	}

	if magnetLink != "" {
		server.downloader.AddTorrent(magnetLink, downloadDir)
		fmt.Fprintf(w, "")
	} else {
		http.Error(w, "Missing Magnet link", http.StatusBadRequest)
	}
}

func files(w http.ResponseWriter, r *http.Request) {
	infoHash := r.URL.Query().Get(":infohash")
	file := r.URL.Query().Get(":file")

	if infoHash != "" {
		if magnet, ok := server.magnets[infoHash]; ok {
			if file != "" {
				r.URL.Path = file
				log.Printf("[HTTP] Serving %s: %s\n", magnet.InfoHash, path.Join(magnet.DownloadDir, file))
				http.FileServer(MagnetFileSystem{magnet}).ServeHTTP(w, r)
			} else {
				log.Println("[HTTP] Listing", magnet.InfoHash)
				routes.ServeJson(w, magnet)
			}
		} else {
			http.Error(w, "Invalid Magnet info hash", http.StatusNotFound)
		}
	} else {
		log.Println("[HTTP] Listing all Magnets")
		routes.ServeJson(w, server.downloader.handles)
	}
}

func shutdown(w http.ResponseWriter, r *http.Request) {
	log.Println("[HTTP] Stopping")
	server.downloader.Stop()
	os.Exit(0)
}
