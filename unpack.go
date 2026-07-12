// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/klauspost/compress/zstd"
)

func unpack(input, output, password string) error {
	in, err := os.Open(input)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(output)
	if err != nil {
		return err
	}
	defer out.Close()

	header, err := readHeader(in)
	if err != nil {
		return err
	}
	key := deriveKey(password, header.Salt[:])

	decoder, err := zstd.NewReader(nil)
	if err != nil {
		return err
	}
	defer decoder.Close()

	for i := uint32(0); i < header.ChunkCount; i++ {
		chunkHeader, err := readChunkHeader(in)
		if err != nil {
			return err
		}
		buf := make([]byte, chunkHeader.CompSize)
		if _, err := io.ReadFull(in, buf); err != nil {
			return err
		}
		dec, err := decrypt(buf, key, chunkHeader.Nonce[:])
		if err != nil {
			return err
		}
		data, err := decoder.DecodeAll(dec, nil)
		if err != nil {
			return err
		}
		if uint32(len(data)) != chunkHeader.OrigSize {
			return fmt.Errorf("chunk %d size mismatch: got %d want %d", chunkHeader.Index, len(data), chunkHeader.OrigSize)
		}
		if _, err := out.Write(data); err != nil {
			return err
		}
	}

	fmt.Println("Unpacked")
	return nil
}
