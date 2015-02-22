package main

import "flag"

type Settings struct {
	httpPort                int
	bitTorrentPort          int
	uPNPNatPMPEnabled       bool
	maxDownloadRate         int
	maxUploadRate           int
	keepFiles               bool
	inactivityPauseTimeout  int
	inactivityRemoveTimeout int
}

var settings Settings

func main() {
	settings = Settings{}
	flag.IntVar(&settings.httpPort, "http-port", 8080, "Port used for HTTP server")
	flag.IntVar(&settings.bitTorrentPort, "bittorrent-port", 6900, "Port used for BitTorrent incoming connections")
	flag.BoolVar(&settings.uPNPNatPMPEnabled, "upnp-natpmp-enabled", true, "Enable UPNP/NATPMP")
	flag.IntVar(&settings.maxDownloadRate, "max-download-rate", 0, "Maximum download rate in kB/s, 0 = Unlimited")
	flag.IntVar(&settings.maxUploadRate, "max-upload-rate", 0, "Maximum upload rate in kB/s, 0 = Unlimited")
	flag.BoolVar(&settings.keepFiles, "keep-files", false, "Keep downloaded files upon stopping")
	flag.IntVar(&settings.inactivityPauseTimeout, "inactivity-pause-timeout", 30, "Torrents will be paused after some inactivity")
	flag.IntVar(&settings.inactivityRemoveTimeout, "inactivity-remove-timeout", 600, "Torrents will be removed after some inactivity")
	flag.Parse()

	bitTorrentStart()
	httpStart()
	httpStop()
	bitTorrentStop()
}
