package main

type MagnetFile struct {
	Name string `json:"name"`
}

func NewMagnetFile(name string) *MagnetFile {
	return &MagnetFile{name}
}

type Magnet struct {
	InfoHash    string        `json:"info_hash"`
	DownloadDir string        `json:"download_dir"`
	Files       []*MagnetFile `json:"files"`
}

func NewMagnet(infoHash string, downloadDir string) *Magnet {
	return &Magnet{InfoHash: infoHash, DownloadDir: downloadDir}
}
