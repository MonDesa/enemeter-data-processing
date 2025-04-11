package main

import (
	"enemeter-data-processing/internal/commands"
	"fmt"
	"os"
)

func main() {
	// Display version and app name
	fmt.Printf("%s version: %s\n", commands.AppName, commands.CurrentVersion)

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Check the command
	switch os.Args[1] {
	case "process":
		// Create and configure the process command
		processCmd := commands.SetupProcessCommand()
		if err := processCmd.Parse(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing arguments: %v\n", err)
			processCmd.Usage()
			os.Exit(1)
		}

		// Parse CLI options and execute the process command
		options := commands.ParseCommandLineOptions(processCmd)
		if err := commands.ProcessCommand(options); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "version":
		fmt.Printf("%s\n", commands.CurrentVersion)

	case "help":
		printUsage()

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

// printUsage displays help information for the CLI tool
func printUsage() {
	fmt.Printf("%s - A tool for processing and analyzing ENEMETER data\n\n", commands.AppName)
	fmt.Println("Usage:")
	fmt.Println("  enemeter-cli <command> [options]")
	fmt.Println("\nAvailable Commands:")
	fmt.Println("  process     Process ENEMETER data files")
	fmt.Println("  version     Show version information")
	fmt.Println("  help        Show help information")
	fmt.Println("\nFor command-specific help:")
	fmt.Println("  enemeter-cli <command> --help")
}
