package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 5 {
		fmt.Println("Usage:")
		fmt.Println("  myz pack input output.myz password")
		fmt.Println("  myz unpack input.myz output password")
		return
	}

	cmd := os.Args[1]

	switch cmd {

	case "pack":
		if err := pack(os.Args[2], os.Args[3], os.Args[4]); err != nil {
			fmt.Println("Error:", err)
		}

	case "unpack":
		if err := unpack(os.Args[2], os.Args[3], os.Args[4]); err != nil {
			fmt.Println("Error:", err)
		}
	default:
		fmt.Println("Unknown command")
	}
}
