package introspect

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

const timeout = 5 * time.Second

type GDB struct {
	conn       net.Conn
	r          *bufio.Reader
	packetSize int
	halted     bool
}

func Connect(addr string) (*GDB, error) {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, err
	}

	gdb := &GDB{
		conn:       conn,
		r:          bufio.NewReader(conn),
		packetSize: 1024,
		halted:     true,
	}

	if err := gdb.setPacketSize(); err != nil {
		return nil, err
	}

	// attaching halts service
	if err := gdb.Resume(); err != nil {
		gdb.conn.Close()
		return nil, err
	}

	return gdb, nil
}

func (gdb *GDB) Close() error {
	gdb.send("D")
	return gdb.conn.Close()
}

func (gdb *GDB) Halt() error {
	if gdb.halted {
		return nil
	}

	if err := gdb.sendBytes([]byte{0x03}); err != nil {
		return err
	}
	if _, err := gdb.receive(); err != nil {
		return err
	}

	gdb.halted = true
	return nil
}

func (gdb *GDB) Resume() error {
	if !gdb.halted {
		return nil
	}

	if err := gdb.send("c"); err != nil {
		return err
	}

	gdb.halted = false
	return nil
}

func (gdb *GDB) Read(addr uint32, length int) ([]byte, error) {
	if !gdb.halted {
		return nil, errors.New("read while not halted")
	}

	chunk := (gdb.packetSize - 4) / 2
	result := make([]byte, 0, length)

	for len(result) < length {
		at := addr + uint32(len(result))
		n := min(chunk, length-len(result))

		msg := fmt.Sprintf("m%x,%x", at, n)
		resp, err := gdb.sendAndReceive(msg)
		if err != nil {
			return nil, err
		}

		if resp == "" || resp[0] == 'E' {
			return nil, errors.New(fmt.Sprintf("read at %s", msg))
		}

		data, err := hex.DecodeString(resp)
		if err != nil {
			return nil, err
		}

		if len(data) != n {
			return nil, errors.New(fmt.Sprintf("short read at %s: wanted %d, got %d", msg, n, len(data)))
		}

		result = append(result, data...)
	}

	return result, nil
}

func (gdb *GDB) setPacketSize() error {
	resp, err := gdb.sendAndReceive("qSupported")
	if err != nil {
		return err
	}

	features := strings.Split(resp, ";")
	for _, feature := range features {
		sizeStr, ok := strings.CutPrefix(feature, "PacketSize=")
		if !ok {
			continue
		}

		size, err := strconv.ParseInt(sizeStr, 16, 32)
		if err == nil {
			gdb.packetSize = int(size)
			return nil
		}
	}

	// use default
	return nil
}

func (gdb *GDB) sendAndReceive(data string) (string, error) {
	if err := gdb.send(data); err != nil {
		return "", err
	}
	return gdb.receive()
}

func (gdb *GDB) send(data string) error {
	msg := fmt.Sprintf("$%s#%02x", data, checksum(data))
	return gdb.sendBytes([]byte(msg))
}

func (gdb *GDB) sendBytes(bytes []byte) error {
	gdb.conn.SetWriteDeadline(time.Now().Add(timeout))

	if _, err := gdb.conn.Write(bytes); err != nil {
		return err
	}

	return nil
}

func (gdb *GDB) receive() (string, error) {
	gdb.conn.SetReadDeadline(time.Now().Add(timeout))

	_, err := gdb.r.ReadString('$')
	if err != nil {
		return "", errors.New(fmt.Sprintf("reading receive: %s", err))
	}

	body, err := gdb.r.ReadString('#')
	if err != nil {
		return "", errors.New(fmt.Sprintf("reading body: %s", err))
	}
	body = body[:len(body)-1]

	var sum [2]byte
	if _, err := io.ReadFull(gdb.r, sum[:]); err != nil {
		return "", errors.New(fmt.Sprintf("reading checksum: %s", err))
	}

	check, err := strconv.ParseUint(string(sum[:]), 16, 8)
	if err != nil || byte(check) != checksum(body) {
		return "", errors.New("validating checksum")
	}

	if err := gdb.sendBytes([]byte("+")); err != nil {
		return "", err
	}

	return rle(body)
}

func rle(data string) (string, error) {
	var s strings.Builder
	var prev byte

	for i := 0; i < len(data); i++ {
		char := data[i]

		if char != '*' {
			s.WriteByte(char)
			prev = char
		} else {
			i += 1
			if i >= len(data) {
				return "", errors.New(fmt.Sprintf("truncated rle: %s", data))
			}

			repeat := int(data[i]) - 29
			if repeat < 0 {
				return "", errors.New(fmt.Sprintf("invalid repetition rle: %s", data))
			}

			str := strings.Repeat(string(prev), repeat)
			s.WriteString(str)
		}
	}

	return s.String(), nil
}

func checksum(data string) byte {
	var sum byte

	for i := 0; i < len(data); i++ {
		sum += data[i]
	}

	return sum
}
