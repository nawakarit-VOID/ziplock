package main

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"os"

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
}

const chunkSize = 1 << 20 // 1MB

func pack(input, output, password string) error {
	in, _ := os.Open(input)
	defer in.Close()

	out, _ := os.Create(output)
	defer out.Close()

	info, _ := in.Stat()

	header := Header{
		Version:   2,
		ChunkSize: chunkSize,
		FileSize:  uint64(info.Size()),
	}

	writeHeader(out, header)

	// 🔐 generate salt + nonce
	var salt [16]byte
	var nonce [12]byte
	rand.Read(salt[:])
	rand.Read(nonce[:])

	encHeader := EncHeader{Salt: salt, Nonce: nonce}
	binary.Write(out, binary.LittleEndian, encHeader)

	key := deriveKey(password, salt[:])

	encoder, _ := zstd.NewWriter(nil)

	jobs := make(chan Job, 4)
	results := make(chan Result, 4)

	// worker
	for w := 0; w < 4; w++ {
		go func() {
			for job := range jobs {
				comp := encoder.EncodeAll(job.data, nil)
				enc, _ := encrypt(comp, key, nonce[:])
				results <- Result{job.index, enc, len(job.data)}
			}
		}()
	}

	// reader
	go func() {
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
		}
		close(jobs)
	}()

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

			binary.Write(out, binary.LittleEndian, uint32(len(r.data)))
			binary.Write(out, binary.LittleEndian, uint32(r.orig))
			out.Write(r.data)

			totalWritten += int64(r.orig)
			fmt.Printf("\rProgress: %.2f%%",
				float64(totalWritten)/float64(info.Size())*100)

			delete(cache, expected)
			expected++
		}

		if totalWritten >= info.Size() {
			break
		}
	}

	fmt.Println("\nDone")
	return nil
}
