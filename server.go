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

type HttpSettings struct {
	port int
}

type BitTorrentSettings struct {
	port         int
	downloadRate int
	uploadRate   int
	keepFiles    bool
}

type Settings struct {
	http       HttpSettings
	bitTorrent BitTorrentSettings
}

type Server struct {
	settings Settings
	magnets  map[string]*Magnet
}

func NewServer() *Server {
	return &Server{magnets: make(map[string]*Magnet)}
}

var server = NewServer()

func (server *Server) Run() {
	log.Println("[HTTP] Starting on port", server.settings.http.port)

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
		magnet := NewMagnet(magnetLink, downloadDir)
		magnet.Files = append(magnet.Files, NewMagnetFile("Guardians of the Galaxy (2014).mp4"))
		server.magnets[magnetLink] = magnet

		log.Printf("[HTTP] Downloading %s to %s\n", magnetLink, downloadDir)
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
		routes.ServeJson(w, server.magnets)
	}
}

func shutdown(w http.ResponseWriter, r *http.Request) {
	log.Println("[HTTP] Shutting down")
	os.Exit(0)
}
