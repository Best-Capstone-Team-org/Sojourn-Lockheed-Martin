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

func LoadAndStartFirmware(firmwareName string, downlinkChan chan []byte) {
	cmd, err := load(firmwareName)

	if err != nil {
		app.ErrorLogger.Fatalf("Error loading emulator: %+v", err)
	}

	go uartReader(downlinkChan)

	app.LoaderLogger.Println("Waiting on firmware...")
	err = cmd.Wait()

	if err != nil {
		app.ErrorLogger.Fatalf("Error waiting for emulator proc: %+v\n", err)
	}
}

func uartReader(downlinkChan chan []byte) {
	address := net.JoinHostPort(UartTCPAddr, strconv.Itoa(UartTCPPort))
	timeout := 5 * time.Second

	app.LoaderLogger.Printf("Connecting to %s", address)
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		app.ErrorLogger.Fatalf("Connection failed: %+v", err)
	}

	for {
		response, err := bufio.NewReader(conn).ReadBytes('\n')

		if err != nil {
			app.ErrorLogger.Printf("Failed to read response: %v\n", err)
			continue
		}

		downlinkChan <- response
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
