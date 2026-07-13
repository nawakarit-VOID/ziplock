// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/klauspost/compress/zstd"
)

type archiveEntry struct {
	relPath string
	absPath string
	isDir   bool
	size    uint64
}

const chunkSize = 1 << 20 // 1MB

func pack(input, output, password string) error {
	entries, rootName, err := collectEntries(input)
	if err != nil {
		return err
	}

	if info, err := os.Stat(output); err == nil && info.IsDir() {
		base := filepath.Base(filepath.Clean(input))
		if base == "." || base == string(filepath.Separator) {
			base = "archive"
		}
		output = filepath.Join(output, base+".ziplock")
	}

	out, err := os.Create(output)
	if err != nil {
		return err
	}
	defer out.Close()

	var salt [16]byte
	if _, err := rand.Read(salt[:]); err != nil {
		return err
	}

	header := makeArchiveHeader(formatVersionV5, chunkSize, uint32(len(entries)), salt)
	if err := writeHeader(out, header); err != nil {
		return err
	}

	key := deriveKey(formatVersionV5, password, salt[:])

	encoder, err := zstd.NewWriter(nil)
	if err != nil {
		return err
	}
	defer encoder.Close()

	totalFiles := int64(0)
	for _, entry := range entries {
		if !entry.isDir {
			totalFiles += int64(entry.size)
		}
	}

	var written int64
	buf := make([]byte, chunkSize)

	for _, entry := range entries {
		path := entry.relPath
		if rootName != "" {
			path = filepath.ToSlash(filepath.Join(rootName, entry.relPath))
		}

		encodedPath := []byte(path)
		entryHeader := EntryHeader{
			Type:             entryTypeDir,
			PathLen:          uint32(len(encodedPath)),
			UncompressedSize: 0,
			ChunkCount:       0,
		}
		if !entry.isDir {
			entryHeader.Type = entryTypeFile
			entryHeader.UncompressedSize = entry.size
			entryHeader.ChunkCount = uint32((entry.size + chunkSize - 1) / chunkSize)
		}

		var headerBuf bytes.Buffer
		if err := binary.Write(&headerBuf, binary.LittleEndian, entryHeader); err != nil {
			return err
		}
		if _, err := headerBuf.Write(encodedPath); err != nil {
			return err
		}

		var nonce [12]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return err
		}

		enc, err := encrypt(headerBuf.Bytes(), key, nonce[:])
		if err != nil {
			return err
		}

		if _, err := out.Write(nonce[:]); err != nil {
			return err
		}
		cipherSize := uint32(len(enc))
		if err := binary.Write(out, binary.LittleEndian, cipherSize); err != nil {
			return err
		}
		if _, err := out.Write(enc); err != nil {
			return err
		}

		if entry.isDir {
			continue
		}

		file, err := os.Open(entry.absPath)
		if err != nil {
			return err
		}

		for chunkIndex := uint32(0); chunkIndex < entryHeader.ChunkCount; chunkIndex++ {
			n, readErr := io.ReadFull(file, buf)
			if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
				if n == 0 {
					break
				}
				readErr = nil
			}
			if readErr != nil {
				file.Close()
				return readErr
			}

			chunkData := make([]byte, n)
			copy(chunkData, buf[:n])

			comp := encoder.EncodeAll(chunkData, nil)
			var nonce [12]byte
			if _, err := rand.Read(nonce[:]); err != nil {
				file.Close()
				return err
			}
			enc, err := encrypt(comp, key, nonce[:])
			if err != nil {
				file.Close()
				return err
			}

			chunkHeader := ChunkHeader{
				Index:    chunkIndex,
				OrigSize: uint32(len(chunkData)),
				CompSize: uint32(len(enc)),
				Nonce:    nonce,
			}
			if err := writeChunkHeader(out, chunkHeader); err != nil {
				file.Close()
				return err
			}
			if _, err := out.Write(enc); err != nil {
				file.Close()
				return err
			}

			written += int64(len(chunkData))
			fmt.Printf("\rProgress: %.2f%%", float64(written)/float64(totalFiles)*100)
		}

		file.Close()
	}

	fmt.Println("\nDone")
	return nil
}

func collectEntries(input string) ([]archiveEntry, string, error) {
	info, err := os.Stat(input)
	if err != nil {
		return nil, "", err
	}

	if !info.IsDir() {
		return []archiveEntry{{
			relPath: filepath.Base(input),
			absPath: input,
			isDir:   false,
			size:    uint64(info.Size()),
		}}, "", nil
	}

	rootName := filepath.Base(filepath.Clean(input))
	var entries []archiveEntry

	err = filepath.WalkDir(input, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == input {
			return nil
		}
		rel, err := filepath.Rel(input, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			entries = append(entries, archiveEntry{
				relPath: rel,
				absPath: path,
				isDir:   true,
			})
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		entries = append(entries, archiveEntry{
			relPath: rel,
			absPath: path,
			isDir:   false,
			size:    uint64(info.Size()),
		})
		return nil
	})
	if err != nil {
		return nil, "", err
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].isDir != entries[j].isDir {
			return entries[i].isDir
		}
		return strings.Compare(entries[i].relPath, entries[j].relPath) < 0
	})

	entries = append([]archiveEntry{{
		relPath: "",
		absPath: input,
		isDir:   true,
	}}, entries...)

	return entries, rootName, nil
}
