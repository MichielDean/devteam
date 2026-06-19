package main

import (
	"fmt"
	"os"
)

const version = "0.1.0-dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "version":
		fmt.Printf("devteam %s\n", version)
	case "status":
		fmt.Println("devteam: no specs in progress (platform not yet implemented)")
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "devteam %s - multi-agent development platform\n\n", version)
	fmt.Fprintf(os.Stderr, "Usage:\n")
	fmt.Fprintf(os.Stderr, "  devteam <command> [args]\n\n")
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  status    Show current pipeline status\n")
	fmt.Fprintf(os.Stderr, "  version   Print version\n\n")
	fmt.Fprintf(os.Stderr, "Self-bootstrapping: spec 001 defines this platform.\n")
}