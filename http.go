package main

import (
	"fmt"
	"log"
	"mime"
	"net/http"
	"regexp"
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
	mux.Get("/", index)
	mux.Get("/video", video)
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

func index(w http.ResponseWriter, r *http.Request) {
	routes.ServeJson(w, httpInstance.bitTorrent.GetTorrentInfos())
}

func video(w http.ResponseWriter, r *http.Request) {
	downloadDir := r.URL.Query().Get("download_dir")
	if downloadDir == "" {
		downloadDir = "."
	}

	preview := r.URL.Query().Get("preview")
	if preview == "" {
		preview = "0"
	}

	if magnetLink := r.URL.Query().Get("magnet_link"); magnetLink != "" {
		if regExpMatch := regexp.MustCompile(`xt=urn:btih:([a-zA-Z0-9]+)`).FindStringSubmatch(magnetLink); len(regExpMatch) == 2 {
			infoHash := regExpMatch[1]

			httpInstance.bitTorrent.AddTorrent(magnetLink, downloadDir)

			if torrentInfo := httpInstance.bitTorrent.GetTorrentInfo(infoHash); torrentInfo != nil {
				httpInstance.bitTorrent.AddConnection(infoHash)
				defer httpInstance.bitTorrent.RemoveConnection(infoHash)

				if torrentFileInfo := torrentInfo.GetBiggestTorrentFileInfo(); torrentFileInfo != nil && torrentFileInfo.CompletePieces > 0 {
					if preview == "0" {
						if torrentFileInfo.Open(torrentInfo.DownloadDir) {
							defer torrentFileInfo.Close()
							log.Print("Video ready: Serving")
							http.ServeContent(w, r, torrentFileInfo.Path, time.Time{}, torrentFileInfo)
						} else {
							http.Error(w, "Failed to open file", http.StatusInternalServerError)
						}
					} else {
						log.Print("Video ready: Sending response")
						videoReady(w, true)
					}
				} else {
					// Video not ready yet
					if preview == "0" {
						log.Print("Video not ready: Redirecting")
						redirect(w, r)
					} else {
						log.Print("Video not ready: Sending response")
						videoReady(w, false)
					}
				}
			} else {
				// Torrent not ready yet
				if preview == "0" {
					log.Print("Torrent not ready: Redirecting")
					redirect(w, r)
				} else {
					log.Print("Torrent not ready: Sending response")
					videoReady(w, false)
				}
			}
		} else {
			http.Error(w, "Invalid Magnet link", http.StatusBadRequest)
		}
	} else {
		http.Error(w, "Missing Magnet link", http.StatusBadRequest)
	}
}

func shutdown(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	httpInstance.server.Stop(500 * time.Millisecond)
}

func redirect(w http.ResponseWriter, r *http.Request) {
	time.Sleep(2 * time.Second)
	http.Redirect(w, r, r.URL.String(), http.StatusTemporaryRedirect)
}

func videoReady(w http.ResponseWriter, videoReady bool) {
	type VideoReadyResponse struct {
		VideoReady bool `json:"video_ready"`
	}
	routes.ServeJson(w, VideoReadyResponse{VideoReady: videoReady})
}
