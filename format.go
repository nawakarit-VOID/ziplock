// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

var (
	magicV1 = []byte("MYZ1")
	magicV2 = []byte("MYZ2")
)

type EncHeader struct {
	Salt  [16]byte
	Nonce [12]byte
}

type Header struct {
	Version    uint8
	Flags      uint8
	Reserved   uint16
	ChunkSize  uint32
	FileSize   uint64
	ChunkCount uint32
	Salt       [16]byte
}

type ChunkHeader struct {
	Index    uint32
	OrigSize uint32
	CompSize uint32
	Nonce    [12]byte
}

const (
	formatVersionV1 = 1
	formatVersionV2 = 2
	formatVersionV3 = 3
	headerSizeV3    = 1 + 1 + 2 + 4 + 8 + 4 + 16
	chunkHeaderSize  = 4 + 4 + 4 + 12
)

var errBadMagic = errors.New("invalid archive magic")

func writeMagic(w io.Writer, magic []byte) error {
	if _, err := w.Write(magic); err != nil {
		return err
	}
	return nil
}

func writeHeader(w io.Writer, h Header) error {
	if err := writeMagic(w, magicV2); err != nil {
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
	if string(buf) != string(magicV2) && string(buf) != string(magicV1) {
		return nil, errBadMagic
	}
	var h Header
	if err := binary.Read(r, binary.LittleEndian, &h); err != nil {
		return nil, err
	}
	return &h, nil
}

func writeChunkHeader(w io.Writer, h ChunkHeader) error {
	return binary.Write(w, binary.LittleEndian, h)
}

func readChunkHeader(r io.Reader) (*ChunkHeader, error) {
	var h ChunkHeader
	if err := binary.Read(r, binary.LittleEndian, &h); err != nil {
		return nil, err
	}
	return &h, nil
}

func makeArchiveHeader(version uint8, chunkSize uint32, fileSize uint64, chunkCount uint32, salt [16]byte) Header {
	return Header{
		Version:    version,
		Flags:      0,
		Reserved:   0,
		ChunkSize:  chunkSize,
		FileSize:   fileSize,
		ChunkCount: chunkCount,
		Salt:       salt,
	}
}

// deriveKey creates the archive key from password and salt.
func deriveKey(password string, salt []byte) []byte {
	return pbkdf2.Key([]byte(password), salt, 100000, 32, sha256.New)
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func encrypt(data, key, nonce []byte) ([]byte, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	return aead.Seal(nil, nonce, data, nil), nil
}

func decrypt(data, key, nonce []byte) ([]byte, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, nonce, data, nil)
}
