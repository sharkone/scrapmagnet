package main

import (
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"regexp"
	"syscall"
	"time"

	"github.com/drone/routes"
	"github.com/stretchr/graceful"
)

var httpInstance *Http = nil

type Http struct {
	bitTorrent *BitTorrent
	server     *graceful.Server
}

func NewHttp(bitTorrent *BitTorrent) *Http {
	mime.AddExtensionType(".avi", "video/avi")
	mime.AddExtensionType(".mkv", "video/x-matroska")
	mime.AddExtensionType(".mp4", "video/mp4")

	mux := routes.New()
	mux.Get("/", index)
	mux.Get("/video", video)
	mux.Get("/shutdown", shutdown)

	return &Http{
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
	// Parent process monitoring
	if settings.parentPID != 1 {
		go func() {
			for {
				parentAlive := true

				p, err := os.FindProcess(settings.parentPID)
				if err != nil {
					parentAlive = false
				} else {
					err := p.Signal(syscall.Signal(0))
					if err != nil {
						parentAlive = false
					}
				}

				if !parentAlive {
					log.Print("Parent process is dead, exiting")
					httpInstance.server.Stop(500 * time.Millisecond)

				}
				time.Sleep(time.Second)
			}
		}()
	}

	httpInstance = h
	h.server.ListenAndServe()
}

func (h *Http) Stop() {
}

func index(w http.ResponseWriter, r *http.Request) {
	routes.ServeJson(w, httpInstance.bitTorrent.GetTorrentInfos())
}

func video(w http.ResponseWriter, r *http.Request) {
	magnetLink := getQueryParam(r, "magnet_link", "")
	downloadDir := getQueryParam(r, "download_dir", ".")
	preview := getQueryParam(r, "preview", "0")

	if magnetLink != "" {
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
							http.ServeContent(w, r, torrentFileInfo.Path, time.Time{}, torrentFileInfo)
						} else {
							http.Error(w, "Failed to open file", http.StatusInternalServerError)
						}
					} else {
						videoReady(w, true)
					}
				} else {
					// Video not ready yet
					if preview == "0" {
						redirect(w, r)
					} else {
						videoReady(w, false)
					}
				}
			} else {
				// Torrent not ready yet
				if preview == "0" {
					redirect(w, r)
				} else {
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

func getQueryParam(r *http.Request, paramName string, defaultValue string) (result string) {
	result = r.URL.Query().Get(paramName)
	if result == "" {
		result = defaultValue
	}
	return result
}

func redirect(w http.ResponseWriter, r *http.Request) {
	time.Sleep(2 * time.Second)
	http.Redirect(w, r, r.URL.String(), http.StatusTemporaryRedirect)
}

func videoReady(w http.ResponseWriter, videoReady bool) {
	routes.ServeJson(w, map[string]interface{}{"video_ready": videoReady})
}
