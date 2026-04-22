package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/binary"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

var magic = []byte("MYZ1")

type EncHeader struct {
	Salt  [16]byte
	Nonce [12]byte
}

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

// 🔑 helper: derive key (PBKDF2 แบบง่าย)
func deriveKey(password string, salt []byte) []byte {
	return pbkdf2.Key([]byte(password), salt, 100000, 32, sha256.New)
}

// 🔐 encrypt/decrypt helper
func encrypt(data, key, nonce []byte) ([]byte, error) {
	block, _ := aes.NewCipher(key)
	aead, _ := cipher.NewGCM(block)
	return aead.Seal(nil, nonce, data, nil), nil
}

func decrypt(data, key, nonce []byte) ([]byte, error) {
	block, _ := aes.NewCipher(key)
	aead, _ := cipher.NewGCM(block)
	return aead.Open(nil, nonce, data, nil)
}
