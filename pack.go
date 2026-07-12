// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"

	"github.com/klauspost/compress/zstd"
)

type Job struct {
	index int
	data  []byte
}

type Result struct {
	index int
	data  []byte
	orig  int
	nonce [12]byte
}

const chunkSize = 1 << 20 // 1MB
const workerCount = 4
const pipelineDepth = workerCount * 2

func pack(input, output, password string) error {
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

	info, err := in.Stat()
	if err != nil {
		return err
	}

	fileSize := uint64(info.Size())
	chunkCount := uint32((fileSize + chunkSize - 1) / chunkSize)

	var salt [16]byte
	if _, err := rand.Read(salt[:]); err != nil {
		return err
	}

	header := makeArchiveHeader(formatVersionV2, chunkSize, fileSize, chunkCount, salt)
	if err := writeHeader(out, header); err != nil {
		return err
	}

	key := deriveKey(password, salt[:])

	jobs := make(chan Job, pipelineDepth)
	results := make(chan Result, pipelineDepth)
	var wg sync.WaitGroup

	workerTotal := workerCount
	if cpu := runtime.NumCPU(); cpu > 0 && cpu < workerTotal {
		workerTotal = cpu
	}
	if workerTotal < 1 {
		workerTotal = 1
	}

	for w := 0; w < workerTotal; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			encoder, err := zstd.NewWriter(nil)
			if err != nil {
				return
			}
			defer encoder.Close()

			for job := range jobs {
				comp := encoder.EncodeAll(job.data, nil)
				var nonce [12]byte
				if _, err := rand.Read(nonce[:]); err != nil {
					continue
				}
				enc, err := encrypt(comp, key, nonce[:])
				if err != nil {
					continue
				}
				results <- Result{index: job.index, data: enc, orig: len(job.data), nonce: nonce}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	buf := make([]byte, chunkSize)
	i := 0
	for {
		n, err := in.Read(buf)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			jobs <- Job{i, data}
			i++
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			close(jobs)
			return err
		}
	}
	close(jobs)

	// writer (ordered)
	expected := 0
	cache := map[int]Result{}

	totalWritten := int64(0)

	for res := range results {
		cache[res.index] = res

		for {
			r, ok := cache[expected]
			if !ok {
				break
			}

			chunkHeader := ChunkHeader{
				Index:    uint32(expected),
				OrigSize: uint32(r.orig),
				CompSize: uint32(len(r.data)),
				Nonce:    r.nonce,
			}
			if err := writeChunkHeader(out, chunkHeader); err != nil {
				return err
			}
			if _, err := out.Write(r.data); err != nil {
				return err
			}

			totalWritten += int64(r.orig)
			fmt.Printf("\rProgress: %.2f%%",
				float64(totalWritten)/float64(fileSize)*100)

			delete(cache, expected)
			expected++
		}

	}

	fmt.Println("\nDone")
	return nil
}
