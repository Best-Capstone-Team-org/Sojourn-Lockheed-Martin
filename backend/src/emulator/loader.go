package emulator

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const qemuSystemArmEnvVar = "QEMU_SYSTEM_ARM_BIN"

const UartTCPAddr = "127.0.0.1"
const UartTCPPort = 5599

func Load(firmwareFilePath string) (*exec.Cmd, error) {
	log.Printf("Loading %s\n", firmwareFilePath)

	if !fileExists(firmwareFilePath) {
		return nil, errors.New(fmt.Sprintf("firmware binary `%s` not found",
			firmwareFilePath))
	}

	log.Printf("Found %s\n", firmwareFilePath)

	qemuSystemARMBinary, err := findQEMUSystemARM()
	if err != nil {
		return nil, err
	}

	log.Printf("Found qemu binary: %s\n", qemuSystemARMBinary)

	qemuArgs := []string{
		"-M", "mps2-an386",
		"-nographic",
		"-kernel", firmwareFilePath,
		"-serial", fmt.Sprintf("tcp:%s:%d,server=on,wait=off", UartTCPAddr, UartTCPPort)}

	log.Printf("Starting qemu: %s %s\n",
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
