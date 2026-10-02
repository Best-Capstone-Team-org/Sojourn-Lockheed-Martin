package emulator

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

type Introspect struct {
	conn       net.Conn
	r          *bufio.Reader
	packetSize int
	halted     bool
}

func Connect(addr string) (*Introspect, error) {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, err
	}

	in := &Introspect{
		conn:       conn,
		r:          bufio.NewReader(conn),
		packetSize: 1024,
		halted:     true,
	}

	if err := in.setPacketSize(); err != nil {
		return nil, err
	}

	// attaching halts service
	if err := in.Resume(); err != nil {
		in.conn.Close()
		return nil, err
	}

	return in, nil
}

func (in *Introspect) Close() error {
	in.send("D")
	return in.conn.Close()
}

func (in *Introspect) Halt() error {
	if in.halted {
		return nil
	}

	if err := in.sendBytes([]byte{0x03}); err != nil {
		return err
	}
	if _, err := in.receive(); err != nil {
		return err
	}

	in.halted = true
	return nil
}

func (in *Introspect) Resume() error {
	if !in.halted {
		return nil
	}

	if err := in.send("c"); err != nil {
		return err
	}

	in.halted = false
	return nil
}

func (in *Introspect) Read(addr uint32, length int) ([]byte, error) {
	if !in.halted {
		return nil, errors.New("read while not halted")
	}

	chunk := (in.packetSize - 4) / 2
	result := make([]byte, 0, length)

	for len(result) < length {
		at := addr + uint32(len(result))
		n := min(chunk, length-len(result))

		msg := fmt.Sprintf("m%x,%x", at, n)
		resp, err := in.sendAndReceive(msg)
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

func (in *Introspect) setPacketSize() error {
	resp, err := in.sendAndReceive("qSupported")
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
			in.packetSize = int(size)
			return nil
		}
	}

	// use default
	return nil
}

func (in *Introspect) sendAndReceive(data string) (string, error) {
	if err := in.send(data); err != nil {
		return "", err
	}
	return in.receive()
}

func (in *Introspect) send(data string) error {
	msg := fmt.Sprintf("$%s#%02x", data, checksum(data))
	return in.sendBytes([]byte(msg))
}

func (in *Introspect) sendBytes(bytes []byte) error {
	in.conn.SetWriteDeadline(time.Now().Add(timeout))

	if _, err := in.conn.Write(bytes); err != nil {
		return err
	}

	return nil
}

func (in *Introspect) receive() (string, error) {
	in.conn.SetReadDeadline(time.Now().Add(timeout))

	_, err := in.r.ReadString('$')
	if err != nil {
		return "", errors.New(fmt.Sprintf("reading receive: %s", err))
	}

	body, err := in.r.ReadString('#')
	if err != nil {
		return "", errors.New(fmt.Sprintf("reading body: %s", err))
	}
	body = body[:len(body)-1]

	var sum [2]byte
	if _, err := io.ReadFull(in.r, sum[:]); err != nil {
		return "", errors.New(fmt.Sprintf("reading checksum: %s", err))
	}

	check, err := strconv.ParseUint(string(sum[:]), 16, 8)
	if err != nil || byte(check) != checksum(body) {
		return "", errors.New("validating checksum")
	}

	if err := in.sendBytes([]byte("+")); err != nil {
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
