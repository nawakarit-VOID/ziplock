// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"io"
	"log"
	"runtime"

	"golang.org/x/crypto/argon2"
)

var magic = []byte("MYZ5")

type ArchiveHeader struct {
	Version    uint8
	Flags      uint8
	Reserved   uint16
	ChunkSize  uint32
	EntryCount uint32
	Comment    string
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
	formatVersion   = 5
	headerSizeBody  = 1 + 1 + 2 + 4 + 4 // Version + Flags + Reserved + ChunkSize + EntryCount
	entryHeaderSize = 1 + 3 + 4 + 8 + 4
	chunkHeaderSize = 4 + 4 + 4 + 12
)

var errBadMagic = errors.New("archive is corrupted")

func archiveCorruptErrorf(format string, args ...any) error {
	log.Printf("archive corruption detected: "+format, args...)
	return errors.New("archive is corrupted")
}

func writeMagic(w io.Writer) error {
	if _, err := w.Write(magic); err != nil {
		return err
	}
	return nil
}

func makeHeaderAAD(magicBytes []byte, salt []byte) []byte {
	aad := make([]byte, len(magicBytes)+len(salt))
	copy(aad, magicBytes)
	copy(aad[len(magicBytes):], salt)
	return aad
}

func writeHeader(w io.Writer, h ArchiveHeader, key []byte) error {
	if err := writeMagic(w); err != nil {
		return err
	}
	if _, err := w.Write(h.Salt[:]); err != nil {
		return err
	}

	var body bytes.Buffer
	if err := binary.Write(&body, binary.LittleEndian, h.Version); err != nil {
		return err
	}
	if err := binary.Write(&body, binary.LittleEndian, h.Flags); err != nil {
		return err
	}
	if err := binary.Write(&body, binary.LittleEndian, h.Reserved); err != nil {
		return err
	}
	if err := binary.Write(&body, binary.LittleEndian, h.ChunkSize); err != nil {
		return err
	}
	if err := binary.Write(&body, binary.LittleEndian, h.EntryCount); err != nil {
		return err
	}
	commentBytes := []byte(h.Comment)
	commentLen := uint32(len(commentBytes))
	if err := binary.Write(&body, binary.LittleEndian, commentLen); err != nil {
		return err
	}
	if _, err := body.Write(commentBytes); err != nil {
		return err
	}

	var nonce [12]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	if _, err := w.Write(nonce[:]); err != nil {
		return err
	}

	aad := makeHeaderAAD(magic, h.Salt[:])
	enc, err := encrypt(body.Bytes(), key, nonce[:], aad)
	if err != nil {
		return err
	}

	cipherSize := uint32(len(enc))
	if err := binary.Write(w, binary.LittleEndian, cipherSize); err != nil {
		return err
	}
	if _, err := w.Write(enc); err != nil {
		return err
	}

	// Compute and append a global HMAC over magic||salt||encrypted_header_ciphertext
	mac := hmac.New(sha256.New, key)
	mac.Write(magic)
	mac.Write(h.Salt[:])
	mac.Write(enc)
	sum := mac.Sum(nil)
	if _, err := w.Write(sum); err != nil {
		return err
	}
	return nil
}

func readHeader(r io.Reader, password string) (*ArchiveHeader, error) {
	buf := make([]byte, 4)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	if subtle.ConstantTimeCompare(buf, magic) != 1 {
		return nil, errBadMagic
	}

	var salt [16]byte
	if _, err := io.ReadFull(r, salt[:]); err != nil {
		return nil, err
	}

	key := deriveKey(password, salt[:])

	var nonce [12]byte
	if _, err := io.ReadFull(r, nonce[:]); err != nil {
		return nil, err
	}

	var cipherSize uint32
	if err := binary.Read(r, binary.LittleEndian, &cipherSize); err != nil {
		return nil, err
	}
	if cipherSize == 0 || cipherSize > 1024 {
		return nil, archiveCorruptErrorf("invalid archive header cipher size: %d", cipherSize)
	}

	enc := make([]byte, cipherSize)
	if _, err := io.ReadFull(r, enc); err != nil {
		return nil, err
	}

	// Read and verify appended HMAC-SHA256 (32 bytes)
	macRead := make([]byte, sha256.Size)
	if _, err := io.ReadFull(r, macRead); err != nil {
		return nil, err
	}

	mac := hmac.New(sha256.New, key)
	mac.Write(magic)
	mac.Write(salt[:])
	mac.Write(enc)
	expected := mac.Sum(nil)
	if !hmac.Equal(macRead, expected) {
		return nil, archiveCorruptErrorf("archive HMAC verification failed")
	}

	aad := makeHeaderAAD(magic, salt[:])
	dec, err := decrypt(enc, key, nonce[:], aad)
	if err != nil {
		return nil, err
	}

	if len(dec) < headerSizeBody+4 {
		return nil, archiveCorruptErrorf("invalid decrypted archive header size: got %d", len(dec))
	}

	var h ArchiveHeader
	decBuf := bytes.NewReader(dec)
	if err := binary.Read(decBuf, binary.LittleEndian, &h.Version); err != nil {
		return nil, err
	}
	if h.Version != formatVersion {
		return nil, archiveCorruptErrorf("invalid archive version in encrypted header: got %d", h.Version)
	}
	if err := binary.Read(decBuf, binary.LittleEndian, &h.Flags); err != nil {
		return nil, err
	}
	if err := binary.Read(decBuf, binary.LittleEndian, &h.Reserved); err != nil {
		return nil, err
	}
	if err := binary.Read(decBuf, binary.LittleEndian, &h.ChunkSize); err != nil {
		return nil, err
	}
	if err := binary.Read(decBuf, binary.LittleEndian, &h.EntryCount); err != nil {
		return nil, err
	}

	var commentLen uint32
	if err := binary.Read(decBuf, binary.LittleEndian, &commentLen); err != nil {
		return nil, err
	}
	if commentLen > 4096 {
		return nil, archiveCorruptErrorf("archive comment length too large: %d", commentLen)
	}
	if decBuf.Len() < int(commentLen) {
		return nil, archiveCorruptErrorf("archive comment data truncated")
	}
	commentBytes := make([]byte, commentLen)
	if _, err := io.ReadFull(decBuf, commentBytes); err != nil {
		return nil, err
	}
	h.Comment = string(commentBytes)
	h.Salt = salt
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

func makeArchiveHeader(chunkSize uint32, entryCount uint32, comment string, salt [16]byte) ArchiveHeader {
	return ArchiveHeader{
		Version:    formatVersion,
		Flags:      0,
		Reserved:   0,
		ChunkSize:  chunkSize,
		EntryCount: entryCount,
		Comment:    comment,
		Salt:       salt,
	}
}

func zeroizeBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// deriveKey creates the archive key from password and salt using Argon2id.
func deriveKey(password string, salt []byte) []byte {
	pwd := []byte(password)
	threads := uint8(runtime.GOMAXPROCS(0))
	if threads < 1 {
		threads = 1
	}
	key := argon2.IDKey(pwd, salt, 1, 64*1024, threads, 32)
	zeroizeBytes(pwd)
	return key
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
