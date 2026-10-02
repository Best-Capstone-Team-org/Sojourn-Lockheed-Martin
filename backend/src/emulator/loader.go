package emulator

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sojourn/app"
	"strconv"
	"strings"
	"sync"
	"time"
)

const qemuSystemArmEnvVar = "QEMU_SYSTEM_ARM_BIN"

const UartTCPAddr = "127.0.0.1"
const UartTCPPort = 10000

const uartConnectTimeout = 5 * time.Second

var globalFirmwareLock sync.Mutex

func LoadAndStartFirmware(
	firmwareName string,
	downlinkChan chan []byte,
	uplinkChan chan []byte,
	done chan struct{},
) {
	globalFirmwareLock.Lock()
	defer globalFirmwareLock.Unlock()

	cmd, err := load(firmwareName)

	if err != nil {
		app.ErrorLogger.Fatalf("Error loading emulator: %+v", err)
	}

	cmdDone := make(chan error, 1)

	go func() {
		cmdDone <- cmd.Wait()
	}()

	address := net.JoinHostPort(UartTCPAddr, strconv.Itoa(UartTCPPort))

	app.LoaderLogger.Printf("Connecting to %s", address)
	conn, err := dialUART(address, uartConnectTimeout, cmdDone)
	if err != nil {
		cmd.Process.Kill()
		app.ErrorLogger.Fatalf("Connection failed: %+v", err)
	}

	go uartReader(conn, downlinkChan)
	go uartWriter(conn, uplinkChan)

	app.LoaderLogger.Println("Waiting on firmware...")

	select {
	case err := <-cmdDone:
		if err != nil {
			app.ErrorLogger.Printf("Firmware process failed: %v", err)
		}

	case <-done:
		if cmd.Process != nil {
			app.LoaderLogger.Printf("Killing firmware")
			cmd.Process.Kill()
		}

		<-cmdDone
	}
}

func dialUART(address string, timeout time.Duration, cmdDone <-chan error) (net.Conn, error) {
	deadline := time.Now().Add(timeout)
	backoff := 10 * time.Millisecond

	for {
		conn, err := net.DialTimeout("tcp", address, time.Until(deadline))
		if err == nil {
			return conn, nil
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, fmt.Errorf("timed out after %s: %w", timeout, err)
		}

		select {
		case exitErr := <-cmdDone:
			return nil, fmt.Errorf("qemu exited before UART was ready: %v", exitErr)
		case <-time.After(min(backoff, remaining)):
		}

		backoff = min(backoff*2, 250*time.Millisecond)
	}
}

func uartReader(conn net.Conn, downlinkChan chan []byte) {
	r := bufio.NewReader(conn)
	for {
		response, err := r.ReadBytes('\n')

		if err != nil {
			if errors.Is(err, io.EOF) {
				app.LoaderLogger.Printf("UART TCP connection closed")
			} else {
				app.ErrorLogger.Printf("Failed to read response: %v\n", err)
			}
			break
		}

		downlinkChan <- response
	}

	close(downlinkChan)

	app.LoaderLogger.Printf("Exiting uart reader")
}

func uartWriter(conn net.Conn, uplinkChan chan []byte) {
	for uplinkMessage := range uplinkChan {
		s := strings.TrimSpace(string(uplinkMessage)) + " "

		nn, err := fmt.Fprintf(conn, "%s*%04X\n", s, crc16(s))
		if err != nil {
			app.ErrorLogger.Printf("Error writing to uart connection: %v (wrote %d bytes)",
				err, nn)
		}
	}

	app.LoaderLogger.Println("Exiting uart writer")
}

func load(firmwareFilePath string) (*exec.Cmd, error) {
	app.LoaderLogger.Printf("Loading %s\n", firmwareFilePath)

	if !fileExists(firmwareFilePath) {
		return nil, errors.New(fmt.Sprintf("firmware binary `%s` not found",
			firmwareFilePath))
	}

	app.LoaderLogger.Printf("Found %s\n", firmwareFilePath)

	qemuSystemARMBinary, err := findQEMUSystemARM()
	if err != nil {
		return nil, err
	}

	app.LoaderLogger.Printf("Found qemu binary: %s\n", qemuSystemARMBinary)

	qemuArgs := []string{
		"-M", "mps2-an386",
		"-nographic",
		"-kernel", firmwareFilePath,
		"-serial", fmt.Sprintf("tcp:%s:%d,server=on,wait=on", UartTCPAddr, UartTCPPort)}

	app.LoaderLogger.Printf("Starting qemu: %s %s\n",
		qemuSystemARMBinary, strings.Join(qemuArgs, " "))

	cmd := exec.Command(qemuSystemARMBinary, qemuArgs...)

	err = cmd.Start()

	return cmd, err
}

func findQEMUSystemARM() (string, error) {
	qemuSystemARMBin := os.Getenv(qemuSystemArmEnvVar)
	if len(qemuSystemARMBin) == 0 {
		qemuSystemARMBin = "qemu-system-arm"
	}

	if fileExists(qemuSystemARMBin) {
		return qemuSystemARMBin, nil
	}

	for _, p := range getPATH() {
		binPath := filepath.Join(p, qemuSystemARMBin)
		if fileExists(binPath) {
			return binPath, nil
		}
	}

	return "", errors.New("Unable to find QEMU system ARM binary")
}

func fileExists(fileName string) bool {
	_, err := os.Stat(fileName)
	return err == nil
}

func getPATH() []string {
	return strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))
}

func crc16(s string) uint16 {
	crc := uint16(0xFFFF)

	for i := 0; i < len(s); i++ {
		crc ^= uint16(s[i]) << 8

		for j := 0; j < 8; j++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}

	return crc
}
