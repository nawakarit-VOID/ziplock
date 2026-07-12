package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/klauspost/compress/zstd"
)

func unpack(input, output, password string) error {
	in, _ := os.Open(input)
	defer in.Close()

	out, _ := os.Create(output)
	defer out.Close()

	_, _ = readHeader(in)

	var encHeader EncHeader
	binary.Read(in, binary.LittleEndian, &encHeader)

	key := deriveKey(password, encHeader.Salt[:])

	decoder, _ := zstd.NewReader(nil)

	for {
		var size uint32
		err := binary.Read(in, binary.LittleEndian, &size)
		if err == io.EOF {
			break
		}

		var orig uint32
		binary.Read(in, binary.LittleEndian, &orig)

		buf := make([]byte, size)
		io.ReadFull(in, buf)

		dec, _ := decrypt(buf, key, encHeader.Nonce[:])
		data, _ := decoder.DecodeAll(dec, nil)

		out.Write(data)
	}

	fmt.Println("Unpacked")
	return nil
}
