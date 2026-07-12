package main

import (
	"fmt"
	"os"
)

// = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
// # Main #
// = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func main() {

	if len(os.Args) == 1 || os.Args[1] == "gui" {
		runGUI()
		return
	}

	if len(os.Args) < 5 {
		fmt.Println("Usage:")
		fmt.Println("  ziplock gui")
		fmt.Println("  ziplock pack input output.myz password")
		fmt.Println("  ziplock unpack input.myz output password")
		return
	}

	switch os.Args[1] {
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
