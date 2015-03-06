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
	Path             string   `json:"path"`
	Size             int64    `json:"size"`
	CompletePieces   int      `json:"complete_pieces"`
	ContiguousPieces int      `json:"contiguous_pieces"`
	TotalPieces      int      `json:"total_pieces"`
	PieceMap         []string `json:"piece_map"`

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
	result.CompletePieces = result.GetCompletePieces(false)
	result.ContiguousPieces = result.GetCompletePieces(true)
	result.TotalPieces = 1 + result.endPiece - result.startPiece
	result.PieceMap = result.GetPieceMap()
	return result
}

func (tfi *TorrentFileInfo) GetPieceIndexFromOffset(offset int64) int {
	pieceIndex := int((tfi.offset + offset) / int64(tfi.handle.Torrent_file().Piece_length()))
	return pieceIndex
}

func (tfi *TorrentFileInfo) GetCompletePieces(contiguous bool) int {
	completePieces := 0
	for i := tfi.startPiece; i <= tfi.endPiece; i++ {
		if tfi.handle.Have_piece(i) {
			completePieces += 1
		} else if contiguous {
			return completePieces
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

	handle         libtorrent.Torrent_handle
	connections    int
	connectionChan chan int
}

func NewTorrentInfo(torrentHandle libtorrent.Torrent_handle) *TorrentInfo {
	result := &TorrentInfo{handle: torrentHandle, connections: 0, connectionChan: make(chan int, 10)}
	result.Refresh()

	go func() {
		for {
			if result.connections == 0 {
				if !result.Paused {
					select {
					case inc := <-result.connectionChan:
						resumeTorrent(result.handle)
						result.connections += inc
						result.Paused = false
					case <-time.After(time.Duration(settings.inactivityPauseTimeout) * time.Second):
						pauseTorrent(result.handle)
						result.Paused = true
					}
				} else {
					select {
					case inc := <-result.connectionChan:
						resumeTorrent(result.handle)
						result.connections += inc
						result.Paused = false
					case <-time.After(time.Duration(settings.inactivityRemoveTimeout) * time.Second):
						removeTorrent(result.handle)
						return
					}
				}
			} else {
				result.connections += <-result.connectionChan
			}
		}
	}()

	return result
}

func (ti *TorrentInfo) Refresh() {
	torrentStatus := ti.handle.Status()

	ti.InfoHash = fmt.Sprintf("%X", torrentStatus.GetInfo_hash().To_string())
	ti.Name = torrentStatus.GetName()
	ti.DownloadDir = torrentStatus.GetSave_path()
	ti.State = int(torrentStatus.GetState())
	ti.StateStr = func(state libtorrent.LibtorrentTorrent_statusState_t) string {
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
	ti.Paused = torrentStatus.GetPaused()
	ti.Progress = torrentStatus.GetProgress()
	ti.DownloadRate = torrentStatus.GetDownload_rate() / 1024
	ti.UploadRate = torrentStatus.GetUpload_rate() / 1024
	ti.Seeds = torrentStatus.GetNum_seeds()
	ti.TotalSeeds = torrentStatus.GetNum_complete()
	ti.Peers = torrentStatus.GetNum_peers()
	ti.TotalPeers = torrentStatus.GetNum_incomplete()

	torrentInfo := ti.handle.Torrent_file()
	if torrentInfo.Swigcptr() != 0 {
		ti.Files = func(torrentInfo libtorrent.Torrent_info) []*TorrentFileInfo {
			result := []*TorrentFileInfo{}
			for i := 0; i < torrentInfo.Files().Num_files(); i++ {
				result = append(result, NewTorrentFileInfo(torrentInfo.Files().File_path(i), torrentInfo.Files().File_size(i), torrentInfo.Files().File_offset(i), ti.handle))
			}
			return result
		}(torrentInfo)
		ti.Size = torrentInfo.Files().Total_size()
		ti.Pieces = torrentInfo.Num_pieces()
	}
}

func (ti *TorrentInfo) GetTorrentFileInfo(filePath string) *TorrentFileInfo {
	for _, torrentFileInfo := range ti.Files {
		if torrentFileInfo.Path == filePath {
			return torrentFileInfo
		}
	}
	return nil
}

var torrentSession libtorrent.Session
var torrentInfos []TorrentInfo = make([]TorrentInfo, 0)

var removeChannel chan bool = make(chan bool, 1)
var deleteChannel chan bool = make(chan bool, 1)

func bitTorrentStart() {
	log.Println("[BITTORRENT] Starting")

	fingerprint := libtorrent.NewFingerprint("LT", libtorrent.LIBTORRENT_VERSION_MAJOR, libtorrent.LIBTORRENT_VERSION_MINOR, 0, 0)
	portRange := libtorrent.NewStd_pair_int_int(settings.bitTorrentPort, settings.bitTorrentPort)
	listenInterface := "0.0.0.0"
	sessionFlags := int(libtorrent.SessionAdd_default_plugins)
	alertMask := int(libtorrent.AlertError_notification | libtorrent.AlertStorage_notification | libtorrent.AlertStatus_notification)

	torrentSession = libtorrent.NewSession(fingerprint, portRange, listenInterface, sessionFlags, alertMask)
	go alertPump()

	sessionSettings := torrentSession.Settings()
	sessionSettings.SetSsl_listen(0)
	sessionSettings.SetAnnounce_to_all_tiers(true)
	sessionSettings.SetAnnounce_to_all_trackers(true)
	sessionSettings.SetConnection_speed(100)
	sessionSettings.SetPeer_connect_timeout(2)
	sessionSettings.SetRate_limit_ip_overhead(true)
	sessionSettings.SetRequest_timeout(5)
	sessionSettings.SetTorrent_connect_boost(100)
	if settings.maxDownloadRate > 0 {
		sessionSettings.SetDownload_rate_limit(settings.maxDownloadRate * 1024)
	}
	if settings.maxUploadRate > 0 {
		sessionSettings.SetUpload_rate_limit(settings.maxUploadRate * 1024)
	}
	torrentSession.Set_settings(sessionSettings)

	proxySettings := libtorrent.NewProxy_settings()
	if settings.proxyType == "SOCKS5" {
		proxySettings.SetHostname(settings.proxyHost)
		proxySettings.SetPort(uint16(settings.proxyPort))
		if settings.proxyUser != "" {
			proxySettings.SetXtype(byte(libtorrent.Proxy_settingsSocks5_pw))
			proxySettings.SetUsername(settings.proxyUser)
			proxySettings.SetPassword(settings.proxyPassword)
		} else {
			proxySettings.SetXtype(byte(libtorrent.Proxy_settingsSocks5))
		}
	}
	torrentSession.Set_proxy(proxySettings)

	encryptionSettings := libtorrent.NewPe_settings()
	encryptionSettings.SetOut_enc_policy(byte(libtorrent.Pe_settingsForced))
	encryptionSettings.SetIn_enc_policy(byte(libtorrent.Pe_settingsForced))
	encryptionSettings.SetAllowed_enc_level(byte(libtorrent.Pe_settingsBoth))
	encryptionSettings.SetPrefer_rc4(true)
	torrentSession.Set_pe_settings(encryptionSettings)

	torrentSession.Start_dht()
	torrentSession.Start_lsd()

	if settings.uPNPNatPMPEnabled {
		log.Println("[BITTORRENT] Starting UPNP/NATPMP")
		torrentSession.Start_upnp()
		torrentSession.Start_natpmp()
	}
}

func bitTorrentStop() {
	for i := 0; i < int(torrentSession.Get_torrents().Size()); i++ {
		removeTorrent(torrentSession.Get_torrents().Get(i))
	}

	if settings.uPNPNatPMPEnabled {
		log.Println("[BITTORRENT] Stopping UPNP/NATPMP")
		torrentSession.Stop_natpmp()
		torrentSession.Stop_upnp()
	}

	torrentSession.Stop_lsd()
	torrentSession.Stop_dht()

	log.Println("[BITTORRENT] Stopping")
}

func addTorrent(magnetLink string, downloadDir string) {
	addTorrentParams := libtorrent.NewAdd_torrent_params()
	addTorrentParams.SetUrl(magnetLink)
	addTorrentParams.SetSave_path(downloadDir)
	addTorrentParams.SetStorage_mode(libtorrent.Storage_mode_sparse)
	addTorrentParams.SetFlags(0)

	torrentSession.Async_add_torrent(addTorrentParams)
}

func removeTorrent(torrentHandle libtorrent.Torrent_handle) {
	removeFlags := 0
	if !settings.keepFiles {
		removeFlags = int(libtorrent.SessionDelete_files)
	}
	torrentSession.Remove_torrent(torrentHandle, removeFlags)
	<-removeChannel

	if removeFlags != 0 {
		<-deleteChannel
	}
}

func pauseTorrent(torrentHandle libtorrent.Torrent_handle) {
	torrentHandle.Pause()
}

func resumeTorrent(torrentHandle libtorrent.Torrent_handle) {
	torrentHandle.Resume()
}

func addTorrentInfo(torrentHandle libtorrent.Torrent_handle) {
	torrentInfos = append(torrentInfos, *NewTorrentInfo(torrentHandle))
}

func removeTorrentInfo(torrentHandle libtorrent.Torrent_handle) {
	for i, torrentInfo := range torrentInfos {
		if torrentInfo.handle.Equal(torrentHandle) {
			torrentInfos = append(torrentInfos[:i], torrentInfos[i+1:]...)
			removeChannel <- true
			break
		}
	}
}

func metadataReceived(torrentHandle libtorrent.Torrent_handle) {
	torrentInfo := torrentHandle.Torrent_file()
	for i := 0; i < torrentInfo.Files().Num_files(); i++ {
		torrentFileInfo := NewTorrentFileInfo(torrentInfo.Files().File_path(i), torrentInfo.Files().File_size(i), torrentInfo.Files().File_offset(i), torrentHandle)
		for j := 0; j <= torrentFileInfo.getLookAhead()*4 && j <= torrentFileInfo.endPiece; j++ {
			torrentFileInfo.handle.Set_piece_deadline(j, 3000+j*1000, 0)
		}
	}
}

func getTorrentInfos() *[]TorrentInfo {
	for i, _ := range torrentInfos {
		torrentInfos[i].Refresh()
	}
	return &torrentInfos
}

func getTorrentInfo(infoHash string) *TorrentInfo {
	for i, _ := range torrentInfos {
		if torrentInfos[i].InfoHash == infoHash {
			torrentInfos[i].Refresh()
			return &torrentInfos[i]
		}
	}
	return nil
}

func alertPump() {
	for {
		if torrentSession.Wait_for_alert(libtorrent.Seconds(1)).Swigcptr() != 0 {
			alert := torrentSession.Pop_alert()
			switch alert.Xtype() {
			case libtorrent.Torrent_added_alertAlert_type:
				log.Printf("[BITTORRENT] %s: %s", alert.What(), alert.Message())
				torrentAddedAlert := libtorrent.SwigcptrTorrent_added_alert(alert.Swigcptr())
				addTorrentInfo(torrentAddedAlert.GetHandle())
			case libtorrent.Torrent_removed_alertAlert_type:
				log.Printf("[BITTORRENT] %s: %s", alert.What(), alert.Message())
				torrentRemovedAlert := libtorrent.SwigcptrTorrent_removed_alert(alert.Swigcptr())
				removeTorrentInfo(torrentRemovedAlert.GetHandle())
			case libtorrent.Torrent_deleted_alertAlert_type:
				log.Printf("[BITTORRENT] %s: %s", alert.What(), alert.Message())
				deleteChannel <- true
			case libtorrent.Torrent_delete_failed_alertAlert_type:
				log.Printf("[BITTORRENT] %s: %s", alert.What(), alert.Message())
				deleteChannel <- false
			case libtorrent.Metadata_received_alertAlert_type:
				log.Printf("[BITTORRENT] %s: %s", alert.What(), alert.Message())
				metadataReceivedAlert := libtorrent.SwigcptrMetadata_received_alert(alert.Swigcptr())
				metadataReceived(metadataReceivedAlert.GetHandle())
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
