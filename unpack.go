package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/klauspost/compress/zstd"
)

func unpack(input, output string) error {
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

	_, err = readHeader(in)
	if err != nil {
		return err
	}

	decoder, _ := zstd.NewReader(nil)

	for {
		var compSize uint32
		err := binary.Read(in, binary.LittleEndian, &compSize)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		var origSize uint32
		if err := binary.Read(in, binary.LittleEndian, &origSize); err != nil {
			return err
		}

		comp := make([]byte, compSize)
		if _, err := io.ReadFull(in, comp); err != nil {
			return err
		}

		data, err := decoder.DecodeAll(comp, nil)
		if err != nil {
			return err
		}

		if _, err := out.Write(data); err != nil {
			return err
		}
	}

	fmt.Println("Unpacked:", output)
	return nil
}
