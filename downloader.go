package main

import (
	"fmt"
	"github.com/sharkone/libtorrent-go"
	"log"
	"time"
)

type TorrentFileInfo struct {
	Name string `json:"name"`
	Size int    `json:"size"`
}

type TorrentInfo struct {
	InfoHash     string            `json:"info_hash"`
	Name         string            `json:"name"`
	DownloadDir  string            `json:download_dir`
	State        int               `json:"state"`
	StateStr     string            `json:"state_str"`
	Files        []TorrentFileInfo `json:"files"`
	Size         int               `json:"size"`
	Progress     float32           `json:"progress"`
	DownloadRate int               `json:"download_rate"`
	UploadRate   int               `json:"upload_rate"`
	Seeds        int               `json:"seeds"`
	TotalSeeds   int               `json:"total_seeds"`
	Peers        int               `json:"peers"`
	TotalPeers   int               `json:"total_peers"`
}

func NewTorrentInfo(torrentHandle libtorrent.Torrent_handle) *TorrentInfo {
	torrentStatus := torrentHandle.Status()

	result := &TorrentInfo{}
	result.InfoHash = fmt.Sprintf("%X", torrentStatus.GetInfo_hash().To_string())
	result.Name = torrentStatus.GetName()
	result.DownloadDir = torrentStatus.GetSave_path()
	result.State = int(torrentStatus.GetState())
	result.StateStr = func(state libtorrent.LibtorrentTorrent_statusState_t) string {
		switch state {
		case libtorrent.Torrent_statusQueued_for_checking:
			return "Queued for checking"
		case libtorrent.Torrent_statusChecking_files:
			return "Checking files"
		case libtorrent.Torrent_statusDownloading_metadata:
			return "Downloading metadata"
		case libtorrent.Torrent_statusDownloading:
			return "Downloading"
		case libtorrent.Torrent_statusFinished:
			return "Finished"
		case libtorrent.Torrent_statusSeeding:
			return "Seeding"
		case libtorrent.Torrent_statusAllocating:
			return "Allocating"
		case libtorrent.Torrent_statusChecking_resume_data:
			return "Checking resume data"
		default:
			return "Unknown"
		}
	}(torrentStatus.GetState())
	result.Progress = torrentStatus.GetProgress()
	result.DownloadRate = torrentStatus.GetDownload_rate() / 1024
	result.UploadRate = torrentStatus.GetUpload_rate() / 1024
	result.Seeds = torrentStatus.GetNum_seeds()
	result.TotalSeeds = torrentStatus.GetNum_complete()
	result.Peers = torrentStatus.GetNum_peers()
	result.TotalPeers = torrentStatus.GetNum_incomplete()

	torrentInfo := torrentHandle.Torrent_file()
	if torrentInfo.Swigcptr() != 0 {
		result.Files = func(torrentInfo libtorrent.Torrent_info) []TorrentFileInfo {
			result := []TorrentFileInfo{}
			for i := 0; i < torrentInfo.Files().Num_files(); i++ {
				result = append(result, TorrentFileInfo{
					Name: torrentInfo.Files().File_path(i),
					Size: int(torrentInfo.Files().File_size(i)),
				})
			}
			return result
		}(torrentInfo)
		result.Size = int(torrentInfo.Files().Total_size())
	}

	return result
}

type Downloader struct {
	settings *Settings
	session  libtorrent.Session
}

func NewDownloader(settings *Settings) *Downloader {
	return &Downloader{settings: settings}
}

func (d *Downloader) GetTorrentInfos() []*TorrentInfo {
	result := []*TorrentInfo{}
	for i := 0; i < int(d.session.Get_torrents().Size()); i++ {
		result = append(result, NewTorrentInfo(d.session.Get_torrents().Get(i)))
	}
	return result
}

func (d *Downloader) GetTorrentInfo(infoHash string) *TorrentInfo {
	for i := 0; i < int(d.session.Get_torrents().Size()); i++ {
		torrentHandle := d.session.Get_torrents().Get(i)
		if fmt.Sprintf("%X", torrentHandle.Info_hash().To_string()) == infoHash {
			return NewTorrentInfo(torrentHandle)
		}
	}
	return nil
}

func (d *Downloader) Start() {
	log.Println("[BITTORRENT] Starting")

	fingerprint := libtorrent.NewFingerprint("LT", libtorrent.LIBTORRENT_VERSION_MAJOR, libtorrent.LIBTORRENT_VERSION_MINOR, 0, 0)
	portRange := libtorrent.NewStd_pair_int_int(d.settings.bitTorrent.port, d.settings.bitTorrent.port)
	listenInterface := "0.0.0.0"
	sessionFlags := int(libtorrent.SessionAdd_default_plugins)
	alertMask := int(libtorrent.AlertError_notification | libtorrent.AlertStorage_notification | libtorrent.AlertStatus_notification)

	d.session = libtorrent.NewSession(fingerprint, portRange, listenInterface, sessionFlags, alertMask)
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
}

func (d *Downloader) Stop() {
	// for _, torrentHandle := range d.handles {
	// 	d.removeTorrent(torrentHandle)
	// }

	// for len(d.handles) > 0 {
	// 	time.Sleep(time.Second)
	// }

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

	d.session.Async_add_torrent(addTorrentParams)
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
			switch alert.Xtype() {
			case libtorrent.Add_torrent_alertAlert_type:
				// Ignore
			case libtorrent.External_ip_alertAlert_type:
				// Ignore
			case libtorrent.Portmap_error_alertAlert_type:
				// Ignore
			case libtorrent.Tracker_error_alertAlert_type:
				// Ignore
			default:
				log.Printf("[BITTORRENT] %s: %s", alert.What(), alert.Message())
			}
		}
	}
}
