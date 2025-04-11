package main

import (
	"fmt"
	"os"

	"rubrion-cli/internal/commands"
)

func main() {
	fmt.Printf("Rubrion CLI version: %s\n", commands.CurrentVersion)

	if len(os.Args) > 1 {
		fmt.Println("Received arguments:", os.Args[1:])
	} else {
		fmt.Println("Usage: rubrion-cli [command] [args]")
	}
}
