package main

import (
	"flag"
)

func main() {
	flag.IntVar(&server.settings.http.port, "http-port", 8080, "Port used for HTTP server")
	flag.IntVar(&server.settings.bitTorrent.port, "torrent-port", 6900, "Port used for BitTorrent incoming connections")
	flag.IntVar(&server.settings.bitTorrent.downloadRate, "torrent-download-rate", 0, "Maximum download rate in kB/s, 0 = Unlimited")
	flag.IntVar(&server.settings.bitTorrent.uploadRate, "torrent-upload-rate", 0, "Maximum upload rate in kB/s, 0 = Unlimited")
	flag.BoolVar(&server.settings.bitTorrent.keepFiles, "torrent-keep-files", false, "Keep downloaded files upon stopping")
	flag.Parse()

	server.Run()
}
