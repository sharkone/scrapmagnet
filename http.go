package main

import (
	"log"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/sharkone/routes"
	"github.com/stretchr/graceful"
)

var httpServer *graceful.Server

func httpStart() {
	mime.AddExtensionType(".avi", "video/avi")
	mime.AddExtensionType(".mkv", "video/x-matroska")
	mime.AddExtensionType(".mp4", "video/mp4")

	mux := routes.New()
	mux.Get("/add", add)
	mux.Get("/files", files)
	mux.Get("/files/:infohash", files)
	mux.Get("/files/:infohash/:filepath(.+)", files)
	mux.Get("/shutdown", shutdown)

	httpServer := &graceful.Server{
		Timeout: 500 * time.Millisecond,
		Server: &http.Server{
			Addr:    ":" + strconv.Itoa(settings.httpPort),
			Handler: mux,
		},
	}

	log.Println("[HTTP] Listening on port", settings.httpPort)
	httpServer.ListenAndServe()
}

func httpStop() {
	log.Println("[HTTP] Stopping")
}

func add(w http.ResponseWriter, r *http.Request) {
	downloadDir := r.URL.Query().Get("download_dir")
	if downloadDir == "" {
		downloadDir = "."
	}

	if magnetLink := r.URL.Query().Get("magnet"); magnetLink != "" {
		addTorrent(magnetLink, downloadDir)
		type AddResponse struct {
			magnetLink  string `json:"magnet_link"`
			downloadDir string `json:"download_dir"`
		}
		routes.ServeJson(w, AddResponse{magnetLink: magnetLink, downloadDir: downloadDir})
	} else {
		http.Error(w, "Missing Magnet link", http.StatusBadRequest)
	}
}

func files(w http.ResponseWriter, r *http.Request) {
	infoHash := r.URL.Query().Get(":infohash")
	filePath := r.URL.Query().Get(":filepath")

	if infoHash != "" {
		if torrentInfo := getTorrentInfo(infoHash); torrentInfo != nil {
			torrentInfo.connectionChan <- 1
			if filePath != "" {
				if torrentFileInfo := torrentInfo.GetTorrentFileInfo(filePath); torrentFileInfo != nil {
					if torrentFileInfo.Open(torrentInfo.DownloadDir) {
						defer torrentFileInfo.Close()
						log.Println("[HTTP] Serving:", filePath)
						http.ServeContent(w, r, filePath, time.Time{}, torrentFileInfo)
					} else {
						http.Error(w, "Failed to open file", http.StatusInternalServerError)
					}
				} else {
					http.NotFound(w, r)
				}
			} else {
				routes.ServeJson(w, torrentInfo)
			}
			torrentInfo.connectionChan <- -1
		} else {
			http.Error(w, "Invalid info hash", http.StatusNotFound)
		}
	} else {
		routes.ServeJson(w, getTorrentInfos())
	}
}

func shutdown(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	httpServer.Stop(500 * time.Millisecond)
}
