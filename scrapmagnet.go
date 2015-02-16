package main

import (
	"flag"
)

type HttpSettings struct {
	port int
}

type BitTorrentSettings struct {
	port              int
	uPNPNatPMPEnabled bool
	maxDownloadRate   int
	maxUploadRate     int
	keepFiles         bool
}

type Settings struct {
	http       HttpSettings
	bitTorrent BitTorrentSettings
}

var server *Server

func main() {
	settings := Settings{}
	flag.IntVar(&settings.http.port, "http-port", 8080, "Port used for HTTP server")
	flag.IntVar(&settings.bitTorrent.port, "torrent-port", 6900, "Port used for BitTorrent incoming connections")
	flag.BoolVar(&settings.bitTorrent.uPNPNatPMPEnabled, "torrent-upnp-natpmp-enabled", true, "Enable UPNP/NATPMP")
	flag.IntVar(&settings.bitTorrent.maxDownloadRate, "torrent-max-download-rate", 0, "Maximum download rate in kB/s, 0 = Unlimited")
	flag.IntVar(&settings.bitTorrent.maxUploadRate, "torrent-max-upload-rate", 0, "Maximum upload rate in kB/s, 0 = Unlimited")
	flag.BoolVar(&settings.bitTorrent.keepFiles, "torrent-keep-files", false, "Keep downloaded files upon stopping")
	flag.Parse()

	server = NewServer(&settings)
	server.Run()
}
