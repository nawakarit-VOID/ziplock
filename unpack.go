// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
)

func unpack(input, output, password string) error {
	in, err := os.Open(input)
	if err != nil {
		return err
	}
	defer in.Close()

	header, err := readHeader(in)
	if err != nil {
		return err
	}

	key := deriveKey(header.Version, password, header.Salt[:])

	decoder, err := zstd.NewReader(nil)
	if err != nil {
		return err
	}
	defer decoder.Close()

	if info, err := os.Stat(output); err == nil && !info.IsDir() {
		return fmt.Errorf("output must be a directory: %s", output)
	}
	if err := os.MkdirAll(output, 0o755); err != nil {
		return err
	}

	for i := uint32(0); i < header.EntryCount; i++ {
		var entryHeader EntryHeader
		var pathBytes []byte

		if header.Version >= formatVersionV4 {
			var nonce [12]byte
			if _, err := io.ReadFull(in, nonce[:]); err != nil {
				return err
			}
			var cipherSize uint32
			if err := binary.Read(in, binary.LittleEndian, &cipherSize); err != nil {
				return err
			}
			enc := make([]byte, cipherSize)
			if _, err := io.ReadFull(in, enc); err != nil {
				return err
			}

			dec, err := decrypt(enc, key, nonce[:])
			if err != nil {
				return err
			}

			if len(dec) < entryHeaderSize {
				return fmt.Errorf("decrypted metadata too short")
			}

			decBuf := bytes.NewReader(dec)
			if err := binary.Read(decBuf, binary.LittleEndian, &entryHeader); err != nil {
				return err
			}
			pathBytes = make([]byte, entryHeader.PathLen)
			if _, err := io.ReadFull(decBuf, pathBytes); err != nil {
				return err
			}
		} else {
			eh, err := readEntryHeader(in)
			if err != nil {
				return err
			}
			entryHeader = *eh

			pathBytes = make([]byte, entryHeader.PathLen)
			if _, err := io.ReadFull(in, pathBytes); err != nil {
				return err
			}
		}

		relPath := sanitizeArchivePath(string(pathBytes))
		if relPath == "" {
			continue
		}

		targetPath := filepath.Join(output, filepath.FromSlash(relPath))

		if entryHeader.Type == entryTypeDir {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}

		out, err := os.Create(targetPath)
		if err != nil {
			return err
		}

		for chunkIndex := uint32(0); chunkIndex < entryHeader.ChunkCount; chunkIndex++ {
			chunkHeader, err := readChunkHeader(in)
			if err != nil {
				out.Close()
				return err
			}

			buf := make([]byte, chunkHeader.CompSize)
			if _, err := io.ReadFull(in, buf); err != nil {
				out.Close()
				return err
			}

			dec, err := decrypt(buf, key, chunkHeader.Nonce[:])
			if err != nil {
				out.Close()
				return err
			}

			data, err := decoder.DecodeAll(dec, nil)
			if err != nil {
				out.Close()
				return err
			}

			if uint32(len(data)) != chunkHeader.OrigSize {
				out.Close()
				return fmt.Errorf("chunk %d size mismatch: got %d want %d", chunkHeader.Index, len(data), chunkHeader.OrigSize)
			}

			if _, err := out.Write(data); err != nil {
				out.Close()
				return err
			}
		}

		if err := out.Close(); err != nil {
			return err
		}
	}

	fmt.Println("Unpacked")
	return nil
}

func sanitizeArchivePath(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	path = filepath.ToSlash(filepath.Clean(path))
	path = strings.TrimPrefix(path, "./")
	if path == "." || path == "/" {
		return ""
	}
	if strings.HasPrefix(path, "../") || strings.Contains(path, "/../") {
		return ""
	}
	return path
}
