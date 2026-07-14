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

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/pbkdf2"
)

var (
	magicV1 = []byte("MYZ1")
	magicV2 = []byte("MYZ2")
	magicV3 = []byte("MYZ3")
	magicV4 = []byte("MYZ4")
	magicV5 = []byte("MYZ5")
)

type ArchiveHeader struct {
	Version    uint8
	Flags      uint8
	Reserved   uint16
	ChunkSize  uint32
	EntryCount uint32
	Salt       [16]byte
}

const (
	entryTypeFile = 1
	entryTypeDir  = 2
)

type EntryHeader struct {
	Type             uint8
	Reserved         [3]byte
	PathLen          uint32
	UncompressedSize uint64
	ChunkCount       uint32
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
	formatVersionV4 = 4
	formatVersionV5 = 5
	headerSizeV3    = 1 + 1 + 2 + 4 + 4 + 16
	entryHeaderSize = 1 + 3 + 4 + 8 + 4
	chunkHeaderSize = 4 + 4 + 4 + 12
)

var errBadMagic = errors.New("invalid archive magic")

func writeMagic(w io.Writer, magic []byte) error {
	if _, err := w.Write(magic); err != nil {
		return err
	}
	return nil
}

func writeHeader(w io.Writer, h ArchiveHeader) error {
	if h.Version == formatVersionV5 {
		if err := writeMagic(w, magicV5); err != nil {
			return err
		}
	} else if h.Version == formatVersionV4 {
		if err := writeMagic(w, magicV4); err != nil {
			return err
		}
	} else {
		if err := writeMagic(w, magicV3); err != nil {
			return err
		}
	}
	if err := binary.Write(w, binary.LittleEndian, h); err != nil {
		return err
	}
	return nil
}

func readHeader(r io.Reader) (*ArchiveHeader, error) {
	buf := make([]byte, 4)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	if string(buf) != string(magicV5) && string(buf) != string(magicV4) && string(buf) != string(magicV3) && string(buf) != string(magicV2) && string(buf) != string(magicV1) {
		return nil, errBadMagic
	}
	var h ArchiveHeader
	if err := binary.Read(r, binary.LittleEndian, &h); err != nil {
		return nil, err
	}
	return &h, nil
}

func writeEntryHeader(w io.Writer, h EntryHeader) error {
	return binary.Write(w, binary.LittleEndian, h)
}

func readEntryHeader(r io.Reader) (*EntryHeader, error) {
	var h EntryHeader
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

func makeArchiveHeader(version uint8, chunkSize uint32, entryCount uint32, salt [16]byte) ArchiveHeader {
	return ArchiveHeader{
		Version:    version,
		Flags:      0,
		Reserved:   0,
		ChunkSize:  chunkSize,
		EntryCount: entryCount,
		Salt:       salt,
	}
}

// deriveKey creates the archive key from password and salt.
func deriveKey(version uint8, password string, salt []byte) []byte {
	if version >= formatVersionV5 {
		return argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	}
	return pbkdf2.Key([]byte(password), salt, 100000, 32, sha256.New)
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func encrypt(data, key, nonce, aad []byte) ([]byte, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	return aead.Seal(nil, nonce, data, aad), nil
}

func decrypt(data, key, nonce, aad []byte) ([]byte, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, nonce, data, aad)
}

func encryptWithAAD(data, key, nonce, aad []byte) ([]byte, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	return aead.Seal(nil, nonce, data, aad), nil
}

func decryptWithAAD(data, key, nonce, aad []byte) ([]byte, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, nonce, data, aad)
}
