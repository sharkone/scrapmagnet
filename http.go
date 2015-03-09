package main

import (
	"fmt"
	"mime"
	"net/http"
	"time"

	"github.com/drone/routes"
	"github.com/stretchr/graceful"
)

var httpInstance *Http = nil

type Http struct {
	settings   *Settings
	bitTorrent *BitTorrent
	server     *graceful.Server
}

func NewHttp(settings *Settings, bitTorrent *BitTorrent) *Http {
	mime.AddExtensionType(".avi", "video/avi")
	mime.AddExtensionType(".mkv", "video/x-matroska")
	mime.AddExtensionType(".mp4", "video/mp4")

	mux := routes.New()
	mux.Get("/add", add)
	mux.Get("/files", files)
	mux.Get("/files/:infohash", files)
	mux.Get("/files/:infohash/:filepath(.+)", files)
	mux.Get("/shutdown", shutdown)

	return &Http{
		settings:   settings,
		bitTorrent: bitTorrent,
		server: &graceful.Server{
			Timeout: 500 * time.Millisecond,
			Server: &http.Server{
				Addr:    fmt.Sprintf("%v:%v", "0.0.0.0", settings.httpPort),
				Handler: mux,
			},
		},
	}
}

func (h *Http) Start() {
	httpInstance = h
	h.server.ListenAndServe()
}

func (h *Http) Stop() {
}

func add(w http.ResponseWriter, r *http.Request) {
	downloadDir := r.URL.Query().Get("download_dir")
	if downloadDir == "" {
		downloadDir = "."
	}

	if magnetLink := r.URL.Query().Get("magnet"); magnetLink != "" {
		httpInstance.bitTorrent.AddTorrent(magnetLink, downloadDir)
		type AddResponse struct {
			magnetLink  string `json:"magnet_link"`
			downloadDir string `json:"download_dir"`
		}
		routes.ServeJson(w, AddResponse{magnetLink: magnetLink, downloadDir: downloadDir})
	} else {
		http.Error(w, "Missing Magnet link", http.StatusBadRequest)
	}
}

func shutdown(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	httpInstance.server.Stop(500 * time.Millisecond)
}

func files(w http.ResponseWriter, r *http.Request) {
	infoHash := r.URL.Query().Get(":infohash")
	filePath := r.URL.Query().Get(":filepath")

	if infoHash != "" {
		if torrentInfo := httpInstance.bitTorrent.GetTorrentInfo(infoHash); torrentInfo != nil {
			httpInstance.bitTorrent.AddConnection(infoHash)
			defer httpInstance.bitTorrent.RemoveConnection(infoHash)
			if filePath != "" {
				if torrentFileInfo := torrentInfo.GetTorrentFileInfo(filePath); torrentFileInfo != nil {
					if torrentFileInfo.Open(torrentInfo.DownloadDir) {
						defer torrentFileInfo.Close()
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
		} else {
			http.Error(w, "Invalid info hash", http.StatusNotFound)
		}
	} else {
		routes.ServeJson(w, httpInstance.bitTorrent.GetTorrentInfos())
	}
}
