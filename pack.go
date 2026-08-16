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

const (
	MaxEntryCount = 1_000_000
	MaxPathLen    = 4096
	MinChunkSize  = 1024
	MaxChunkSize  = 16 * 1024 * 1024         // 16MB
	MaxFileSize   = 100 * 1024 * 1024 * 1024 // 100GB
)

func cleanupOutputFile(out *os.File, outputPath string) {
	if out != nil {
		_ = out.Close()
	}
	if outputPath != "" {
		_ = os.Remove(outputPath)
	}
}

func cleanupOnSecureRandomError(err error, out *os.File, outputPath, message string) error {
	if err == nil {
		return nil
	}
	cleanupOutputFile(out, outputPath)
	return fmt.Errorf("%s: %w", message, err)
}

func pack(input, output, password, comment string, progressCb func(percent float64)) error {
	entries, rootName, err := collectEntries(input)
	if err != nil {
		return err
	}

	outputPath := output
	if info, err := os.Stat(output); err == nil && info.IsDir() {
		base := filepath.Base(filepath.Clean(input))
		if base == "." || base == string(filepath.Separator) {
			base = "archive"
		}
		output = filepath.Join(output, base+".ziplock")
	}
	outputPath = output

	out, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer out.Close()

	var salt [16]byte
	if _, err := rand.Read(salt[:]); err != nil {
		return cleanupOnSecureRandomError(err, out, outputPath, "failed to generate secure random salt")
	}

	// Header sanity checks
	if chunkSize < MinChunkSize || chunkSize > MaxChunkSize {
		return fmt.Errorf("invalid chunk size: %d", chunkSize)
	}
	if uint64(len(entries)) > MaxEntryCount {
		return fmt.Errorf("entry count exceeds limit: %d", len(entries))
	}

	header := makeArchiveHeader(chunkSize, uint32(len(entries)), comment, salt)
	key := deriveKey(password, salt[:])
	if err := writeHeader(out, header, key); err != nil {
		cleanupOutputFile(out, outputPath)
		return fmt.Errorf("failed to write archive header: %w", err)
	}

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

	for entryIndex, entry := range entries {
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

		// Entry sanity checks
		if entryHeader.Type != entryTypeFile && entryHeader.Type != entryTypeDir {
			return fmt.Errorf("invalid entry type: %d", entryHeader.Type)
		}
		if entryHeader.PathLen > MaxPathLen {
			return fmt.Errorf("entry path length too long: %d", entryHeader.PathLen)
		}
		if entryHeader.UncompressedSize > MaxFileSize {
			return fmt.Errorf("entry size too large: %d", entryHeader.UncompressedSize)
		}
		expectedChunks := uint64(0)
		if entryHeader.UncompressedSize > 0 {
			expectedChunks = (entryHeader.UncompressedSize + uint64(chunkSize) - 1) / uint64(chunkSize)
		}
		if entryHeader.ChunkCount != uint32(expectedChunks) {
			return fmt.Errorf("chunk count mismatch: got %d, expected %d", entryHeader.ChunkCount, expectedChunks)
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
			return cleanupOnSecureRandomError(err, out, outputPath, "failed to generate secure random nonce")
		}

		var metadataAAD bytes.Buffer
		_ = binary.Write(&metadataAAD, binary.LittleEndian, uint32(entryIndex))
		_, _ = metadataAAD.Write(salt[:])

		enc, err := encrypt(headerBuf.Bytes(), key, nonce[:], metadataAAD.Bytes())
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

		info, err := os.Stat(entry.absPath)
		if err != nil {
			if os.IsNotExist(err) || os.IsPermission(err) {
				// If a file vanished after collection (e.g. temporary lock file), return clean error
				return fmt.Errorf("file vanished or inaccessible (%s): %w", entry.absPath, err)
			}
			return fmt.Errorf("failed to stat file %s: %w", entry.absPath, err)
		}
		if info.IsDir() {
			continue
		}

		file, err := os.Open(entry.absPath)
		if err != nil {
			return fmt.Errorf("failed to open file %s: %w", entry.absPath, err)
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
				return cleanupOnSecureRandomError(err, out, outputPath, "failed to generate secure random nonce")
			}

			// predictedCompSize = len(compressed) + 16-byte AES-GCM tag.
			// AES-GCM always appends exactly 16 bytes, so this prediction is
			// deterministic and matches what unpack reads from ChunkHeader.CompSize.
			predictedCompSize := uint32(len(comp) + 16)
			var chunkAAD bytes.Buffer
			_ = binary.Write(&chunkAAD, binary.LittleEndian, uint32(entryIndex))
			_ = binary.Write(&chunkAAD, binary.LittleEndian, chunkIndex)
			_ = binary.Write(&chunkAAD, binary.LittleEndian, uint32(len(chunkData)))
			_ = binary.Write(&chunkAAD, binary.LittleEndian, predictedCompSize)
			_, _ = chunkAAD.Write(salt[:])

			enc, err := encrypt(comp, key, nonce[:], chunkAAD.Bytes())
			if err != nil {
				file.Close()
				return err
			}

			// Verify our prediction was correct (catches any future AEAD implementation changes)
			if uint32(len(enc)) != predictedCompSize {
				file.Close()
				return fmt.Errorf("internal: encrypted chunk size %d != predicted %d", len(enc), predictedCompSize)
			}

			chunkHeader := ChunkHeader{
				Index:    chunkIndex,
				OrigSize: uint32(len(chunkData)),
				CompSize: predictedCompSize, // same value used in AAD
				Nonce:    nonce,
			}
			// Chunk sanity checks
			if chunkHeader.OrigSize > uint32(chunkSize) {
				return fmt.Errorf("chunk original size exceeds chunk size: %d > %d", chunkHeader.OrigSize, chunkSize)
			}
			// Allow overhead for zstd compression framing (max 64KB) + AES-GCM tag (16 bytes)
			maxAllowed := chunkHeader.OrigSize + 65536 + 16
			if chunkHeader.CompSize > maxAllowed {
				return fmt.Errorf("chunk compressed size too large: %d > %d", chunkHeader.CompSize, maxAllowed)
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
			pct := float64(written) / float64(totalFiles) * 100
			if pct > 100 {
				pct = 100
			}
			fmt.Printf("\rProgress: %.2f%%", pct)
			if progressCb != nil {
				progressCb(pct)
			}
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
		// Use Lstat to safely check symlinks, lock files, and special files
		fi, lstatErr := os.Lstat(path)
		if lstatErr != nil {
			// Skip files that vanished or cannot be lstated (e.g. dynamic lock files)
			return nil
		}

		// Skip symbolic links (like lock files in profile directories) or socket/device files if needed
		if fi.Mode()&os.ModeSymlink != 0 || fi.Mode()&os.ModeSocket != 0 || fi.Mode()&os.ModeNamedPipe != 0 {
			return nil
		}

		rel, err := filepath.Rel(input, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		if fi.IsDir() {
			if _, err := os.ReadDir(path); err != nil {
				if os.IsNotExist(err) || os.IsPermission(err) {
					return nil
				}
				return err
			}
			entries = append(entries, archiveEntry{
				relPath: rel,
				absPath: path,
				isDir:   true,
			})
			return nil
		}

		entries = append(entries, archiveEntry{
			relPath: rel,
			absPath: path,
			isDir:   false,
			size:    uint64(fi.Size()),
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
