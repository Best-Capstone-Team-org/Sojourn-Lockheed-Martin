package emulator

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sojourn/app"
	"strconv"
	"strings"
	"time"
)

const qemuSystemArmEnvVar = "QEMU_SYSTEM_ARM_BIN"

const UartTCPAddr = "127.0.0.1"
const UartTCPPort = 5599

func LoadAndStartFirmware(
	firmwareName string,
	downlinkChan chan []byte,
	uplinkChan chan []byte,
) {
	cmd, err := load(firmwareName)

	if err != nil {
		app.ErrorLogger.Fatalf("Error loading emulator: %+v", err)
	}

	address := net.JoinHostPort(UartTCPAddr, strconv.Itoa(UartTCPPort))
	timeout := 5 * time.Second

	app.LoaderLogger.Printf("Connecting to %s", address)
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		app.ErrorLogger.Fatalf("Connection failed: %+v", err)
	}

	go uartReader(conn, downlinkChan)
	go uartWriter(conn, uplinkChan)

	app.LoaderLogger.Println("Waiting on firmware...")
	err = cmd.Wait()

	if err != nil {
		app.ErrorLogger.Fatalf("Error waiting for emulator proc: %+v\n", err)
	}
}

func uartReader(conn net.Conn, downlinkChan chan []byte) {
	r := bufio.NewReader(conn)
	for {
		response, err := r.ReadBytes('\n')

		if err != nil {
			app.ErrorLogger.Printf("Failed to read response: %v\n", err)
			continue
		}

		downlinkChan <- response
	}
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
		"-serial", fmt.Sprintf("tcp:%s:%d,server=on,wait=off", UartTCPAddr, UartTCPPort)}

	app.LoaderLogger.Printf("Starting qemu: %s %s\n",
		qemuSystemARMBinary, strings.Join(qemuArgs, " "))

	cmd := exec.Command(qemuSystemARMBinary, qemuArgs...)

	err = cmd.Start()

	time.Sleep(time.Second)

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
