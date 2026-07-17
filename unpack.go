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

const maxTotalChunkMem = 2 << 30 // 2 GiB

func parseEntryMetadata(dec []byte) (EntryHeader, []byte, error) {
	if len(dec) < entryHeaderSize {
		return EntryHeader{}, nil, archiveCorruptErrorf("decrypted metadata too short: got %d bytes", len(dec))
	}

	decBuf := bytes.NewReader(dec)
	var entryHeader EntryHeader
	if err := binary.Read(decBuf, binary.LittleEndian, &entryHeader); err != nil {
		return EntryHeader{}, nil, err
	}

	remaining := decBuf.Len()
	if int(entryHeader.PathLen) != remaining {
		return EntryHeader{}, nil, archiveCorruptErrorf("decrypted metadata size mismatch: PathLen=%d but %d bytes remain", entryHeader.PathLen, remaining)
	}

	pathBytes := make([]byte, entryHeader.PathLen)
	if _, err := io.ReadFull(decBuf, pathBytes); err != nil {
		return EntryHeader{}, nil, err
	}

	return entryHeader, pathBytes, nil
}

func writeOutputFile(path string, data io.Reader) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}

	if _, err := io.Copy(tmp, data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return nil
}

func unpack(input, output, password string) error {
	in, err := os.Open(input)
	if err != nil {
		return err
	}
	defer in.Close()

	header, err := readHeader(in, password)
	if err != nil {
		return err
	}

	// ArchiveHeader Sanity Checks
	if header.Version != formatVersion {
		return archiveCorruptErrorf("unsupported archive version: %d", header.Version)
	}
	if header.ChunkSize < 1024 || header.ChunkSize > 16*1024*1024 {
		return archiveCorruptErrorf("invalid archive chunk size: %d bytes", header.ChunkSize)
	}
	if header.EntryCount > 1000000 {
		return archiveCorruptErrorf("archive entry count exceeds hard limit: %d", header.EntryCount)
	}

	key := deriveKey(password, header.Salt[:])

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

	var totalAllocated uint64

	for i := uint32(0); i < header.EntryCount; i++ {
		var entryHeader EntryHeader
		var pathBytes []byte

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

		var metadataAADBuf bytes.Buffer
		_ = binary.Write(&metadataAADBuf, binary.LittleEndian, uint32(i))
		_, _ = metadataAADBuf.Write(header.Salt[:])
		metadataAAD := metadataAADBuf.Bytes()

		dec, err := decrypt(enc, key, nonce[:], metadataAAD)
		if err != nil {
			return err
		}

		entryHeader, pathBytes, err = parseEntryMetadata(dec)
		if err != nil {
			return err
		}

		// EntryHeader Sanity Checks
		if entryHeader.Type != entryTypeFile && entryHeader.Type != entryTypeDir {
			return archiveCorruptErrorf("invalid entry type: %d", entryHeader.Type)
		}
		if entryHeader.PathLen == 0 || entryHeader.PathLen > 4096 {
			return archiveCorruptErrorf("invalid entry path length: %d bytes", entryHeader.PathLen)
		}
		if entryHeader.Type == entryTypeDir {
			if entryHeader.UncompressedSize != 0 {
				return archiveCorruptErrorf("directory entry cannot have non-zero size: %d", entryHeader.UncompressedSize)
			}
			if entryHeader.ChunkCount != 0 {
				return archiveCorruptErrorf("directory entry cannot have non-zero chunk count: %d", entryHeader.ChunkCount)
			}
		} else {
			if entryHeader.UncompressedSize > 100*1024*1024*1024*1024 { // 100TB
				return archiveCorruptErrorf("file size exceeds limit: %d", entryHeader.UncompressedSize)
			}
			expectedChunks := uint32(0)
			if entryHeader.UncompressedSize > 0 {
				expectedChunks = uint32((entryHeader.UncompressedSize + uint64(header.ChunkSize) - 1) / uint64(header.ChunkSize))
			}
			if entryHeader.ChunkCount != expectedChunks {
				return archiveCorruptErrorf("invalid chunk count: got %d want %d", entryHeader.ChunkCount, expectedChunks)
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
		if info, err := os.Stat(targetPath); err == nil && info.IsDir() {
			if err := os.RemoveAll(targetPath); err != nil {
				return err
			}
		}

		var chunkData bytes.Buffer
		for chunkIndex := uint32(0); chunkIndex < entryHeader.ChunkCount; chunkIndex++ {
			chunkHeader, err := readChunkHeader(in)
			if err != nil {
				return err
			}

			// ChunkHeader Sanity Checks
			if chunkHeader.Index != chunkIndex {
				return archiveCorruptErrorf("invalid chunk index: got %d want %d", chunkHeader.Index, chunkIndex)
			}
			if chunkHeader.OrigSize > header.ChunkSize {
				return archiveCorruptErrorf("chunk original size %d exceeds archive chunk size %d", chunkHeader.OrigSize, header.ChunkSize)
			}
			maxCompSize := chunkHeader.OrigSize*2 + 65536
			if chunkHeader.CompSize > maxCompSize {
				return archiveCorruptErrorf("chunk compressed size %d exceeds hard limit %d", chunkHeader.CompSize, maxCompSize)
			}

			// Track total compressed memory requested to avoid OOM from large/poisoned headers.
			totalAllocated += uint64(chunkHeader.CompSize)
			if totalAllocated > maxTotalChunkMem {
				return archiveCorruptErrorf("archive exceeds memory safety limit: %d bytes requested", totalAllocated)
			}

			buf := make([]byte, chunkHeader.CompSize)
			if _, err := io.ReadFull(in, buf); err != nil {
				return err
			}

			var chunkAADBuf bytes.Buffer
			_ = binary.Write(&chunkAADBuf, binary.LittleEndian, uint32(i))
			_ = binary.Write(&chunkAADBuf, binary.LittleEndian, chunkHeader.Index)
			_ = binary.Write(&chunkAADBuf, binary.LittleEndian, chunkHeader.OrigSize)
			_ = binary.Write(&chunkAADBuf, binary.LittleEndian, chunkHeader.CompSize)
			_, _ = chunkAADBuf.Write(header.Salt[:])
			chunkAAD := chunkAADBuf.Bytes()

			dec, err := decrypt(buf, key, chunkHeader.Nonce[:], chunkAAD)
			if err != nil {
				return err
			}

			data, err := decoder.DecodeAll(dec, nil)
			if err != nil {
				return err
			}

			if uint32(len(data)) != chunkHeader.OrigSize {
				return archiveCorruptErrorf("chunk %d size mismatch: got %d want %d", chunkHeader.Index, len(data), chunkHeader.OrigSize)
			}

			if _, err := chunkData.Write(data); err != nil {
				return err
			}
		}

		if err := writeOutputFile(targetPath, bytes.NewReader(chunkData.Bytes())); err != nil {
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
