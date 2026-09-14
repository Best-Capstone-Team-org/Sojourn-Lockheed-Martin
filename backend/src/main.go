package main

import (
	"fmt"
	"log"
	"os"
	"sojourn/emulator"
)

func main() {
	programName := os.Args[0]

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s firmware\n", programName)
		os.Exit(1)
	}

	firmwareName := os.Args[1]

	cmd, err := emulator.Load(firmwareName)

	if err != nil {
		log.Fatalf("Error loading emulator: %+v\n", err)
	}

	log.Println("Waiting on firmware...")
	err = cmd.Wait()

	if err != nil {
		log.Fatalf("Error waiting for emulator proc: %+v\n", err)
	}
}
