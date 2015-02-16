package main

import (
	"github.com/sharkone/libtorrent-go"
	"log"
	"time"
)

type Downloader struct {
	settings *Settings
	session  libtorrent.Session
	handles  []libtorrent.Torrent_handle
}

func NewDownloader(settings *Settings) *Downloader {
	return &Downloader{settings: settings, handles: []libtorrent.Torrent_handle{}}
}

func (d *Downloader) Start() {
	log.Println("[BITTORRENT] Listening on port", d.settings.bitTorrent.port)
	d.session = libtorrent.NewSession()
	d.session.Set_alert_mask(uint(libtorrent.AlertError_notification | libtorrent.AlertStorage_notification | libtorrent.AlertStatus_notification))
	go d.alertPump()

	sessionSettings := d.session.Settings()
	sessionSettings.SetAnnounce_to_all_tiers(true)
	sessionSettings.SetAnnounce_to_all_trackers(true)
	sessionSettings.SetConnection_speed(100)
	sessionSettings.SetPeer_connect_timeout(2)
	sessionSettings.SetRate_limit_ip_overhead(true)
	sessionSettings.SetRequest_timeout(5)
	sessionSettings.SetTorrent_connect_boost(100)

	if d.settings.bitTorrent.maxDownloadRate > 0 {
		sessionSettings.SetDownload_rate_limit(d.settings.bitTorrent.maxDownloadRate * 1024)
	}
	if d.settings.bitTorrent.maxUploadRate > 0 {
		sessionSettings.SetUpload_rate_limit(d.settings.bitTorrent.maxUploadRate * 1024)
	}

	d.session.Set_settings(sessionSettings)

	encryptionSettings := libtorrent.NewPe_settings()
	encryptionSettings.SetOut_enc_policy(byte(libtorrent.Pe_settingsForced))
	encryptionSettings.SetIn_enc_policy(byte(libtorrent.Pe_settingsForced))
	encryptionSettings.SetAllowed_enc_level(byte(libtorrent.Pe_settingsBoth))
	encryptionSettings.SetPrefer_rc4(true)
	d.session.Set_pe_settings(encryptionSettings)

	d.session.Start_dht()
	d.session.Start_lsd()

	if d.settings.bitTorrent.uPNPNatPMPEnabled {
		log.Println("[BITTORRENT] Starting UPNP/NATPMP")
		d.session.Start_upnp()
		d.session.Start_natpmp()
	}

	errorCode := libtorrent.NewError_code()
	d.session.Listen_on(libtorrent.NewStd_pair_int_int(d.settings.bitTorrent.port, d.settings.bitTorrent.port), errorCode)
	if errorCode.Value() != 0 {
		log.Fatal("[BITTORRENT] Failed to open listen socket: %s\n", errorCode.Message())
	}
}

func (d *Downloader) Stop() {
	for _, torrentHandle := range d.handles {
		d.removeTorrent(torrentHandle)
	}

	for len(d.handles) > 0 {
		time.Sleep(time.Second)
	}

	time.Sleep(time.Second * 5)

	if d.settings.bitTorrent.uPNPNatPMPEnabled {
		log.Println("[BITTORRENT] Stopping UPNP/NATPMP")
		d.session.Stop_natpmp()
		d.session.Stop_upnp()
	}

	d.session.Stop_lsd()
	d.session.Stop_dht()

	log.Println("[BITTORRENT] Stopping")
}

func (d *Downloader) AddTorrent(magnetLink string, downloadDir string) {
	addTorrentParams := libtorrent.NewAdd_torrent_params()
	addTorrentParams.SetUrl(magnetLink)
	addTorrentParams.SetSave_path(downloadDir)
	addTorrentParams.SetStorage_mode(libtorrent.Storage_mode_sparse)
	addTorrentParams.SetFlags(uint64(libtorrent.Add_torrent_paramsFlag_sequential_download))

	errorCode := libtorrent.NewError_code()
	torrentHandle := d.session.Add_torrent(addTorrentParams, errorCode)
	if torrentHandle.Is_valid() {
		found := false
		for _, existingHandle := range d.handles {
			if existingHandle.Info_hash() == torrentHandle.Info_hash() {
				found = true
				break
			}
		}

		if !found {
			d.handles = append(d.handles, torrentHandle)
		}
	}
}

func (d *Downloader) removeTorrent(torrentHandle libtorrent.Torrent_handle) {
	removeFlags := 0
	if !d.settings.bitTorrent.keepFiles {
		removeFlags = int(libtorrent.SessionDelete_files)
	}
	d.session.Remove_torrent(torrentHandle, removeFlags)
}

func (d *Downloader) alertPump() {
	for {
		if d.session.Wait_for_alert(libtorrent.Seconds(1)).Swigcptr() != 0 {
			alert := d.session.Pop_alert()
			switch alert.What() {
			case "torrent_removed_alert":
				//panic: interface conversion: libtorrent.SwigcptrAlert is not libtorrent.Torrent_alert: missing method GetHandle
				torrentHandle := alert.(libtorrent.Torrent_alert).GetHandle()
				if d.settings.bitTorrent.keepFiles {
					for i, existingHandle := range d.handles {
						if existingHandle == torrentHandle {
							d.handles = append(d.handles[:i], d.handles[i+1:]...)
							break
						}
					}
				}
				log.Printf("[BITTORRENT] %s: %s", alert.What(), alert.Message())
			case "cache_flushed_alert":
				// ignore
			case "external_ip_alert":
				// ignore
			case "portmap_error_alert":
				// Ignore
			case "tracker_error_alert":
				// Ignore
			default:
				log.Printf("[BITTORRENT] %s: %s", alert.What(), alert.Message())
			}
		}
	}
}
