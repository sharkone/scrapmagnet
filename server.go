package main

import (
	"github.com/drone/routes"
	"github.com/stretchr/graceful"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"
)

type Server struct {
	settings   *Settings
	http       *graceful.Server
	downloader *Downloader
}

func NewServer(settings *Settings) *Server {
	return &Server{settings: settings, downloader: NewDownloader(settings)}
}

func (s *Server) Run() {
	s.downloader.Start()

	mime.AddExtensionType(".avi", "video/avi")
	mime.AddExtensionType(".mkv", "video/x-matroska")
	mime.AddExtensionType(".mp4", "video/mp4")

	mux := routes.New()
	mux.Get("/add", add)
	mux.Get("/files", files)
	mux.Get("/files/:infohash", files)
	mux.Get("/files/:infohash/:filepath(.+)", files)
	mux.Get("/shutdown", shutdown)

	s.http = &graceful.Server{
		Timeout: 500 * time.Millisecond,
		Server: &http.Server{
			Addr:    ":" + strconv.Itoa(server.settings.http.port),
			Handler: mux,
		},
	}

	log.Println("[HTTP] Listening on port", s.settings.http.port)
	s.http.ListenAndServe()
	log.Println("[HTTP] Stopping")

	s.downloader.Stop()
}

func add(w http.ResponseWriter, r *http.Request) {
	magnetLink := r.URL.Query().Get("magnet")

	downloadDir := r.URL.Query().Get("download_dir")
	if downloadDir == "" {
		downloadDir = "."
	}

	if magnetLink != "" {
		server.downloader.AddTorrent(magnetLink, downloadDir)
		w.WriteHeader(http.StatusOK)
	} else {
		http.Error(w, "Missing Magnet link", http.StatusBadRequest)
	}
}

func files(w http.ResponseWriter, r *http.Request) {
	infoHash := r.URL.Query().Get(":infohash")
	filePath := r.URL.Query().Get(":filepath")

	if infoHash != "" {
		torrentInfo := server.downloader.GetTorrentInfo(infoHash)
		if torrentInfo != nil {
			if filePath != "" {
				file, err := os.Open(path.Join(torrentInfo.DownloadDir, filePath))
				if err != nil {
					http.Error(w, "File not found", http.StatusNotFound)
					return
				}
				defer file.Close()

				log.Println("[HTTP] Serving:", filePath)
				http.ServeContent(w, r, filePath, time.Time{}, file)
			} else {
				routes.ServeJson(w, torrentInfo)
			}
		} else {
			http.Error(w, "Invalid info hash", http.StatusNotFound)
			return
		}
	} else {
		routes.ServeJson(w, server.downloader.GetTorrentInfos())
	}
}

func shutdown(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	server.http.Stop(500 * time.Millisecond)
}
