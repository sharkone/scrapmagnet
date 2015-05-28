package main

import (
	"flag"
	"log"
)

type Settings struct {
	parentPID               int
	httpPort                int
	bitTorrentPort          int
	uPNPNatPMPEnabled       bool
	maxDownloadRate         int
	maxUploadRate           int
	keepFiles               bool
	inactivityPauseTimeout  int
	inactivityRemoveTimeout int
	proxyType               string
	proxyHost               string
	proxyPort               int
	proxyUser               string
	proxyPassword           string
	mixpanelToken           string
	mixpanelData            string
}

var (
	settings   = Settings{}
	bitTorrent *BitTorrent
	httpServer *Http
)

func main() {
	flag.IntVar(&settings.parentPID, "ppid", -1, "Parent PID to monitor and auto-shutdown")
	flag.IntVar(&settings.httpPort, "http-port", 8042, "Port used for HTTP server")
	flag.IntVar(&settings.bitTorrentPort, "bittorrent-port", 6900, "Port used for BitTorrent incoming connections")
	flag.BoolVar(&settings.uPNPNatPMPEnabled, "upnp-natpmp-enabled", true, "Enable UPNP/NATPMP")
	flag.IntVar(&settings.maxDownloadRate, "max-download-rate", 0, "Maximum download rate in kB/s, 0 = Unlimited")
	flag.IntVar(&settings.maxUploadRate, "max-upload-rate", 0, "Maximum upload rate in kB/s, 0 = Unlimited")
	flag.BoolVar(&settings.keepFiles, "keep-files", false, "Keep downloaded files upon stopping")
	flag.IntVar(&settings.inactivityPauseTimeout, "inactivity-pause-timeout", 4, "Torrents will be paused after some inactivity")
	flag.IntVar(&settings.inactivityRemoveTimeout, "inactivity-remove-timeout", 600, "Torrents will be removed after some inactivity")
	flag.StringVar(&settings.proxyType, "proxy-type", "None", "Proxy type: None/SOCKS5")
	flag.StringVar(&settings.proxyHost, "proxy-host", "", "Proxy host (ex: myproxy.com, 1.2.3.4")
	flag.IntVar(&settings.proxyPort, "proxy-port", 1080, "Proxy port")
	flag.StringVar(&settings.proxyUser, "proxy-user", "", "Proxy user")
	flag.StringVar(&settings.proxyPassword, "proxy-password", "", "Proxy password")
	flag.StringVar(&settings.mixpanelToken, "mixpanel-token", "", "Mixpanel token")
	flag.StringVar(&settings.mixpanelData, "mixpanel-data", "", "Mixpanel data")
	flag.Parse()

	bitTorrent = NewBitTorrent()
	httpServer = NewHttp(bitTorrent)

	log.Print("[scrapmagnet] Starting")
	bitTorrent.Start()
	httpServer.Start()
	httpServer.Stop()
	bitTorrent.Stop()
	log.Print("[scrapmagnet] Stopping")
}
