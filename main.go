// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"fmt"
	"os"
	"runtime"
)

func init() {
	numCPU := runtime.NumCPU()
	threads := numCPU - 1
	if threads < 1 {
		threads = 1
	}
	runtime.GOMAXPROCS(threads)
}

// = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
// # Main #
// = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func main() {

	if len(os.Args) == 1 || os.Args[1] == "gui" {
		runGUI()
		return
	}

	if len(os.Args) < 5 {
		fmt.Println("Usage:")
		fmt.Println("  ziplock gui")
		fmt.Println("  ziplock pack input output.ziplock password")
		fmt.Println("  ziplock unpack input.ziplock output password")
		return
	}

	switch os.Args[1] {
	case "pack":
		if err := pack(os.Args[2], os.Args[3], os.Args[4], nil); err != nil {
			fmt.Println("Error:", err)
		}
	case "unpack":
		if err := unpack(os.Args[2], os.Args[3], os.Args[4], nil); err != nil {
			fmt.Println("Error:", err)
		}
	default:
		fmt.Println("Unknown command")
	}
}
