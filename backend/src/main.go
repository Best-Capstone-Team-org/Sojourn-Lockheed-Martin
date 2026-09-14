package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"sojourn/emulator"
	"time"
)

func uartReader() {
	address := fmt.Sprintf("%s:%d", emulator.UartTCPAddr, emulator.UartTCPPort)
	timeout := 5 * time.Second

	log.Printf("Connecting to %s", address)
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		log.Fatalf("Connection failed: %+v", err)
	}

	for {
		response, err := bufio.NewReader(conn).ReadString('\n')

		if err != nil {
			log.Printf("Failed to read response: %v\n", err)
			continue
		}

		fmt.Printf("firmware message: %s", response)
	}
}

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

	time.Sleep(time.Second)

	go uartReader()

	log.Println("Waiting on firmware...")
	err = cmd.Wait()

	if err != nil {
		log.Fatalf("Error waiting for emulator proc: %+v\n", err)
	}
}
