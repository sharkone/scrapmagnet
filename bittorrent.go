package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"path"
	"time"

	"github.com/sharkone/libtorrent-go"
)

type TorrentFileInfo struct {
	Path           string   `json:"path"`
	Size           int64    `json:"size"`
	CompletePieces int      `json:"complete_pieces"`
	TotalPieces    int      `json:"total_pieces"`
	PieceMap       []string `json:"piece_map"`

	handle     libtorrent.Torrent_handle
	offset     int64
	startPiece int
	endPiece   int
	file       *os.File
}

func NewTorrentFileInfo(path string, size int64, offset int64, handle libtorrent.Torrent_handle) *TorrentFileInfo {
	result := &TorrentFileInfo{}
	result.Path = path
	result.Size = size
	result.offset = offset
	result.handle = handle
	result.startPiece = result.GetPieceIndexFromOffset(0)
	result.endPiece = result.GetPieceIndexFromOffset(size)
	result.CompletePieces = result.GetCompletePieces()
	result.TotalPieces = 1 + result.endPiece - result.startPiece
	result.PieceMap = result.GetPieceMap()
	return result
}

func (tfi *TorrentFileInfo) GetPieceIndexFromOffset(offset int64) int {
	pieceIndex := int((tfi.offset + offset) / int64(tfi.handle.Torrent_file().Piece_length()))
	return pieceIndex
}

func (tfi *TorrentFileInfo) GetCompletePieces() int {
	completePieces := 0
	for i := tfi.startPiece; i <= tfi.endPiece; i++ {
		if tfi.handle.Have_piece(i) {
			completePieces += 1
		}
	}
	return completePieces
}

func (tfi *TorrentFileInfo) GetPieceMap() []string {
	totalRows := tfi.TotalPieces / 100
	if (tfi.TotalPieces % 100) != 0 {
		totalRows++
	}

	result := make([]string, totalRows)
	for i := tfi.startPiece; i <= tfi.endPiece; i++ {
		if tfi.handle.Have_piece(i) {
			result[(i-tfi.startPiece)/100] += "*"
		} else {
			result[(i-tfi.startPiece)/100] += fmt.Sprintf("%v", tfi.handle.Piece_priority(i))
		}
	}
	return result
}

func (tfi *TorrentFileInfo) Open(downloadDir string) bool {
	if tfi.file == nil {
		fullpath := path.Join(downloadDir, tfi.Path)

		for {
			if _, err := os.Stat(fullpath); err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		tfi.file, _ = os.Open(fullpath)
	}

	return tfi.file != nil
}

func (tfi *TorrentFileInfo) Close() {
	if tfi.file != nil {
		tfi.file.Close()
	}
}

func (tfi *TorrentFileInfo) Read(data []byte) (int, error) {
	totalRead := 0
	size := len(data)

	for size > 0 {
		readSize := int64(math.Min(float64(size), float64(tfi.handle.Torrent_file().Piece_length())))

		currentPosition, _ := tfi.file.Seek(0, os.SEEK_CUR)
		pieceIndex := tfi.GetPieceIndexFromOffset(currentPosition + readSize)
		tfi.waitForPiece(pieceIndex, false)

		tmpData := make([]byte, readSize)
		read, err := tfi.file.Read(tmpData)
		if err != nil {
			totalRead += read
			log.Println("[BITTORRENT]", tfi.file.Fd(), "Read failed!", read, readSize, currentPosition, err)
			return totalRead, err
		}

		copy(data[totalRead:], tmpData[:read])
		totalRead += read
		size -= read
	}

	return totalRead, nil
}

func (tfi *TorrentFileInfo) Seek(offset int64, whence int) (int64, error) {
	newPosition := int64(0)

	switch whence {
	case os.SEEK_SET:
		newPosition = offset
	case os.SEEK_CUR:
		currentPosition, _ := tfi.file.Seek(0, os.SEEK_CUR)
		newPosition = currentPosition + offset
	case os.SEEK_END:
		newPosition = tfi.Size + offset
	}

	pieceIndex := tfi.GetPieceIndexFromOffset(newPosition)
	tfi.waitForPiece(pieceIndex, true)

	ret, err := tfi.file.Seek(offset, whence)
	if err != nil || ret != newPosition {
		log.Println("[BITTORRENT]", tfi.file.Fd(), "Seek failed", ret, newPosition, err)
	}

	return ret, err
}

func (tfi *TorrentFileInfo) waitForPiece(pieceIndex int, timeCritical bool) bool {
	if !tfi.handle.Have_piece(pieceIndex) {
		if timeCritical {
			tfi.handle.Clear_piece_deadlines()
			for i := 0; i <= tfi.getLookAhead() && (i+pieceIndex) <= tfi.endPiece; i++ {
				tfi.handle.Set_piece_deadline(pieceIndex+i, 3000+i*1000, 0)
			}
		} else {
			for i := tfi.startPiece; i < tfi.endPiece; i++ {
				tfi.handle.Piece_priority(i, 1)
			}
			for i := 0; i <= tfi.getLookAhead()*4 && (i+pieceIndex) <= tfi.endPiece; i++ {
				tfi.handle.Piece_priority(pieceIndex+i, 7)
			}
		}

		for {
			time.Sleep(100 * time.Millisecond)
			if tfi.handle.Have_piece(pieceIndex) {
				break
			}
		}
	}

	return false
}

func (tfi *TorrentFileInfo) getLookAhead() int {
	return int(float32(tfi.TotalPieces) * 0.005)
}

type TorrentInfo struct {
	InfoHash     string             `json:"info_hash"`
	Name         string             `json:"name"`
	DownloadDir  string             `json:"download_dir"`
	State        int                `json:"state"`
	StateStr     string             `json:"state_str"`
	Paused       bool               `json:"paused"`
	Files        []*TorrentFileInfo `json:"files"`
	Size         int64              `json:"size"`
	Pieces       int                `json:"pieces"`
	Progress     float32            `json:"progress"`
	DownloadRate int                `json:"download_rate"`
	UploadRate   int                `json:"upload_rate"`
	Seeds        int                `json:"seeds"`
	TotalSeeds   int                `json:"total_seeds"`
	Peers        int                `json:"peers"`
	TotalPeers   int                `json:"total_peers"`
}

func NewTorrentInfo(handle libtorrent.Torrent_handle) (result *TorrentInfo) {
	result = &TorrentInfo{ /*connections: 0, connectionChan: make(chan int, 10)*/ }

	torrentStatus := handle.Status()

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
	result.Paused = torrentStatus.GetPaused()
	result.Progress = torrentStatus.GetProgress()
	result.DownloadRate = torrentStatus.GetDownload_rate() / 1024
	result.UploadRate = torrentStatus.GetUpload_rate() / 1024
	result.Seeds = torrentStatus.GetNum_seeds()
	result.TotalSeeds = torrentStatus.GetNum_complete()
	result.Peers = torrentStatus.GetNum_peers()
	result.TotalPeers = torrentStatus.GetNum_incomplete()

	torrentInfo := handle.Torrent_file()
	if torrentInfo.Swigcptr() != 0 {
		result.Files = func(torrentInfo libtorrent.Torrent_info) (result []*TorrentFileInfo) {
			for i := 0; i < torrentInfo.Files().Num_files(); i++ {
				result = append(result, NewTorrentFileInfo(torrentInfo.Files().File_path(i), torrentInfo.Files().File_size(i), torrentInfo.Files().File_offset(i), handle))
			}
			return result
		}(torrentInfo)
		result.Size = torrentInfo.Files().Total_size()
		result.Pieces = torrentInfo.Num_pieces()
	}
	return result
}

func (ti *TorrentInfo) GetTorrentFileInfo(filePath string) *TorrentFileInfo {
	for _, torrentFileInfo := range ti.Files {
		if torrentFileInfo.Path == filePath {
			return torrentFileInfo
		}
	}
	return nil
}

type BitTorrent struct {
	settings *Settings
	session  libtorrent.Session

	connectionChans map[string]chan int
	connections     map[string]int
	paused          map[string]bool

	removeChan chan bool
	deleteChan chan bool
}

func NewBitTorrent(settings *Settings) *BitTorrent {
	return &BitTorrent{
		settings:        settings,
		connectionChans: make(map[string]chan int),
		connections:     make(map[string]int),
		paused:          make(map[string]bool),
		removeChan:      make(chan bool, 1),
		deleteChan:      make(chan bool, 1),
	}
}

func (b *BitTorrent) Start() {
	log.Println("[BITTORRENT] Starting")

	fingerprint := libtorrent.NewFingerprint("LT", libtorrent.LIBTORRENT_VERSION_MAJOR, libtorrent.LIBTORRENT_VERSION_MINOR, 0, 0)
	portRange := libtorrent.NewStd_pair_int_int(b.settings.bitTorrentPort, b.settings.bitTorrentPort)
	listenInterface := "0.0.0.0"
	sessionFlags := int(libtorrent.SessionAdd_default_plugins)
	alertMask := int(libtorrent.AlertError_notification | libtorrent.AlertStorage_notification | libtorrent.AlertStatus_notification)

	b.session = libtorrent.NewSession(fingerprint, portRange, listenInterface, sessionFlags, alertMask)
	go b.alertPump()

	sessionSettings := b.session.Settings()
	sessionSettings.SetSsl_listen(0)
	sessionSettings.SetAnnounce_to_all_tiers(true)
	sessionSettings.SetAnnounce_to_all_trackers(true)
	sessionSettings.SetConnection_speed(100)
	sessionSettings.SetPeer_connect_timeout(2)
	sessionSettings.SetRate_limit_ip_overhead(true)
	sessionSettings.SetRequest_timeout(5)
	sessionSettings.SetTorrent_connect_boost(100)
	if b.settings.maxDownloadRate > 0 {
		sessionSettings.SetDownload_rate_limit(b.settings.maxDownloadRate * 1024)
	}
	if b.settings.maxUploadRate > 0 {
		sessionSettings.SetUpload_rate_limit(b.settings.maxUploadRate * 1024)
	}
	b.session.Set_settings(sessionSettings)

	proxySettings := libtorrent.NewProxy_settings()
	if b.settings.proxyType == "SOCKS5" {
		proxySettings.SetHostname(b.settings.proxyHost)
		proxySettings.SetPort(uint16(b.settings.proxyPort))
		if b.settings.proxyUser != "" {
			proxySettings.SetXtype(byte(libtorrent.Proxy_settingsSocks5_pw))
			proxySettings.SetUsername(b.settings.proxyUser)
			proxySettings.SetPassword(b.settings.proxyPassword)
		} else {
			proxySettings.SetXtype(byte(libtorrent.Proxy_settingsSocks5))
		}
	}
	b.session.Set_proxy(proxySettings)

	encryptionSettings := libtorrent.NewPe_settings()
	encryptionSettings.SetOut_enc_policy(byte(libtorrent.Pe_settingsForced))
	encryptionSettings.SetIn_enc_policy(byte(libtorrent.Pe_settingsForced))
	encryptionSettings.SetAllowed_enc_level(byte(libtorrent.Pe_settingsBoth))
	encryptionSettings.SetPrefer_rc4(true)
	b.session.Set_pe_settings(encryptionSettings)

	b.session.Start_dht()
	b.session.Start_lsd()

	if b.settings.uPNPNatPMPEnabled {
		log.Println("[BITTORRENT] Starting UPNP/NATPMP")
		b.session.Start_upnp()
		b.session.Start_natpmp()
	}
}

func (b *BitTorrent) Stop() {
	for i := 0; i < int(b.session.Get_torrents().Size()); i++ {
		b.removeTorrent(b.session.Get_torrents().Get(i))
	}

	if b.settings.uPNPNatPMPEnabled {
		log.Println("[BITTORRENT] Stopping UPNP/NATPMP")
		b.session.Stop_natpmp()
		b.session.Stop_upnp()
	}

	b.session.Stop_lsd()
	b.session.Stop_dht()

	log.Println("[BITTORRENT] Stopping")
}

func (b *BitTorrent) AddTorrent(magnetLink string, downloadDir string) {
	addTorrentParams := libtorrent.NewAdd_torrent_params()
	addTorrentParams.SetUrl(magnetLink)
	addTorrentParams.SetSave_path(downloadDir)
	addTorrentParams.SetStorage_mode(libtorrent.Storage_mode_sparse)
	addTorrentParams.SetFlags(0)

	b.session.Async_add_torrent(addTorrentParams)
}

func (b *BitTorrent) GetTorrentInfos() (result []*TorrentInfo) {
	result = make([]*TorrentInfo, 0, 0)
	handles := b.session.Get_torrents()
	for i := 0; i < int(handles.Size()); i++ {
		result = append(result, NewTorrentInfo(handles.Get(i)))
	}
	return result
}

func (b *BitTorrent) GetTorrentInfo(infoHash string) *TorrentInfo {
	handles := b.session.Get_torrents()
	for i := 0; i < int(handles.Size()); i++ {
		handle := handles.Get(i)
		if infoHash == fmt.Sprintf("%X", handle.Info_hash().To_string()) {
			return NewTorrentInfo(handle)
		}
	}
	return nil
}

func (b *BitTorrent) AddConnection(infoHash string) {
	b.connectionChans[infoHash] <- 1
}

func (b *BitTorrent) RemoveConnection(infoHash string) {
	b.connectionChans[infoHash] <- -1
}

func (b *BitTorrent) pauseTorrent(handle libtorrent.Torrent_handle) {
	handle.Pause()
}

func (b *BitTorrent) resumeTorrent(handle libtorrent.Torrent_handle) {
	handle.Resume()
}

func (b *BitTorrent) removeTorrent(handle libtorrent.Torrent_handle) {
	removeFlags := 0
	if !b.settings.keepFiles {
		removeFlags |= int(libtorrent.SessionDelete_files)
	}

	b.session.Remove_torrent(handle, removeFlags)
	<-b.removeChan

	if (removeFlags & int(libtorrent.SessionDelete_files)) != 0 {
		<-b.deleteChan
	}
}

func (b *BitTorrent) alertPump() {
	for {
		if b.session.Wait_for_alert(libtorrent.Seconds(1)).Swigcptr() != 0 {
			alert := b.session.Pop_alert()
			switch alert.Xtype() {
			case libtorrent.Torrent_added_alertAlert_type:
				log.Printf("[BITTORRENT] %s: %s", alert.What(), alert.Message())
				torrentAddedAlert := libtorrent.SwigcptrTorrent_added_alert(alert.Swigcptr())
				b.onTorrentAdded(torrentAddedAlert.GetHandle())
			case libtorrent.Torrent_removed_alertAlert_type:
				log.Printf("[BITTORRENT] %s: %s", alert.What(), alert.Message())
				torrentRemovedAlert := libtorrent.SwigcptrTorrent_removed_alert(alert.Swigcptr())
				b.onTorrentRemoved(torrentRemovedAlert.GetHandle())
			case libtorrent.Torrent_deleted_alertAlert_type:
				log.Printf("[BITTORRENT] %s: %s", alert.What(), alert.Message())
				b.deleteChan <- true
			case libtorrent.Torrent_delete_failed_alertAlert_type:
				log.Printf("[BITTORRENT] %s: %s", alert.What(), alert.Message())
				b.deleteChan <- false
			case libtorrent.Add_torrent_alertAlert_type:
				// Ignore
			case libtorrent.Cache_flushed_alertAlert_type:
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

func (b *BitTorrent) onTorrentAdded(handle libtorrent.Torrent_handle) {
	go func() {
		infoHash := fmt.Sprintf("%X", handle.Info_hash().To_string())

		b.connectionChans[infoHash] = make(chan int)
		b.connections[infoHash] = 0
		b.paused[infoHash] = false

		for {
			if b.connections[infoHash] == 0 {
				if !b.paused[infoHash] {
					select {
					case inc := <-b.connectionChans[infoHash]:
						b.resumeTorrent(handle)
						b.connections[infoHash] += inc
						b.paused[infoHash] = false
					case <-time.After(time.Duration(b.settings.inactivityPauseTimeout) * time.Second):
						b.pauseTorrent(handle)
						b.paused[infoHash] = true
					}
				} else {
					select {
					case inc := <-b.connectionChans[infoHash]:
						b.resumeTorrent(handle)
						b.connections[infoHash] += inc
						b.paused[infoHash] = false
					case <-time.After(time.Duration(b.settings.inactivityRemoveTimeout) * time.Second):
						b.removeTorrent(handle)
						return
					}
				}
			} else {
				b.connections[infoHash] += <-b.connectionChans[infoHash]
			}
		}
	}()
}

func (b *BitTorrent) onTorrentRemoved(handle libtorrent.Torrent_handle) {
	infoHash := fmt.Sprintf("%X", handle.Info_hash().To_string())
	delete(b.connectionChans, infoHash)
	delete(b.connections, infoHash)
	delete(b.paused, infoHash)
	b.removeChan <- true
}
