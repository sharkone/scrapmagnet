package main

import "net"

type NetWriter struct {
	network string
	address string
}

func NewNetWriter(network string, address string) *NetWriter {
	return &NetWriter{network: network, address: address}
}

func (w *NetWriter) Write(p []byte) (int, error) {
	if conn, err := net.Dial(w.network, w.address); err != nil {
		return 0, err
	} else {
		defer conn.Close()
		return conn.Write(p)
	}
}
