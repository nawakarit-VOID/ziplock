package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/klauspost/compress/zstd"
)

const chunkSize = 1 << 20 // 1MB

func pack(input, output string) error {
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

	info, _ := in.Stat()

	header := Header{
		Version:   1,
		ChunkSize: chunkSize,
		FileSize:  uint64(info.Size()),
	}

	if err := writeHeader(out, header); err != nil {
		return err
	}

	encoder, _ := zstd.NewWriter(nil)

	buf := make([]byte, chunkSize)

	for {
		n, err := in.Read(buf)
		if n > 0 {
			comp := encoder.EncodeAll(buf[:n], nil)

			// write sizes
			if err := binary.Write(out, binary.LittleEndian, uint32(len(comp))); err != nil {
				return err
			}
			if err := binary.Write(out, binary.LittleEndian, uint32(n)); err != nil {
				return err
			}

			// write data
			if _, err := out.Write(comp); err != nil {
				return err
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	fmt.Println("Packed:", output)
	return nil
}
