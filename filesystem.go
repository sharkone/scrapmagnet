package main

import (
	"errors"
	"net/http"
	"strings"
)

type MagnetFileSystem struct {
	magnet *Magnet
}

func (mfs MagnetFileSystem) Open(name string) (http.File, error) {
	for _, file := range mfs.magnet.Files {
		if file.Name == strings.TrimPrefix(name, "/") {
			return http.Dir(mfs.magnet.DownloadDir).Open(file.Name)
		}
	}

	return nil, errors.New("File not found in Magnet")
}
