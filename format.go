package main

import (
	"encoding/binary"
	"io"
)

var magic = []byte("MYZ1")

type Header struct {
	Version   uint8
	ChunkSize uint32
	FileSize  uint64
}

type ChunkMeta struct {
	Offset   uint64
	CompSize uint32
	OrigSize uint32
}

func writeHeader(w io.Writer, h Header) error {
	if _, err := w.Write(magic); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, h); err != nil {
		return err
	}
	return nil
}

func readHeader(r io.Reader) (*Header, error) {
	buf := make([]byte, 4)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	if string(buf) != "MYZ1" {
		return nil, io.ErrUnexpectedEOF
	}
	var h Header
	if err := binary.Read(r, binary.LittleEndian, &h); err != nil {
		return nil, err
	}
	return &h, nil
}
