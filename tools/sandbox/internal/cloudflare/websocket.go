package cloudflare

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
)

const (
	opContinuation = 0x0
	opText         = 0x1
	opBinary       = 0x2
	opClose        = 0x8
	opPing         = 0x9
	opPong         = 0xa

	finBit              = 0x80
	maskBit             = 0x80
	maxFramePayload     = 32 * 1024
	maxControlPayload   = 125
	webSocketAcceptGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
)

type webSocket struct {
	conn        net.Conn
	reader      *bufio.Reader
	writeLock   sync.Mutex
	unreadBytes uint64
}

func dialWebSocket(ctx context.Context, host string) (*webSocket, error) {
	dialer := tls.Dialer{Config: &tls.Config{ServerName: host, NextProtos: []string{"http/1.1"}}}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, "443"))
	if err != nil {
		return nil, fmt.Errorf("connect to the tunnel: %w", err)
	}

	socket, err := upgrade(conn, host)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return socket, nil
}

func upgrade(conn net.Conn, host string) (*webSocket, error) {
	nonce := make([]byte, 16)
	rand.Read(nonce)
	key := base64.StdEncoding.EncodeToString(nonce)

	request := "GET / HTTP/1.1\r\n" +
		"Host: " + host + "\r\n" +
		"Connection: Upgrade\r\n" +
		"Upgrade: websocket\r\n" +
		"Sec-WebSocket-Version: 13\r\n" +
		"Sec-WebSocket-Key: " + key + "\r\n" +
		"User-Agent: sbx\r\n\r\n"
	if _, err := io.WriteString(conn, request); err != nil {
		return nil, fmt.Errorf("open the tunnel: %w", err)
	}

	reader := bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, nil)
	if err != nil {
		return nil, fmt.Errorf("open the tunnel: %w", err)
	}
	if response.StatusCode != http.StatusSwitchingProtocols {
		return nil, fmt.Errorf("the tunnel answered %s", response.Status)
	}

	accept := sha1.Sum([]byte(key + webSocketAcceptGUID))
	if response.Header.Get("Sec-WebSocket-Accept") != base64.StdEncoding.EncodeToString(accept[:]) {
		return nil, fmt.Errorf("the tunnel answered with a bad WebSocket handshake")
	}
	return &webSocket{conn: conn, reader: reader}, nil
}

func (w *webSocket) Read(p []byte) (int, error) {
	for w.unreadBytes == 0 {
		opcode, length, err := w.readHeader()
		if err != nil {
			return 0, err
		}

		switch opcode {
		case opContinuation, opText, opBinary:
			w.unreadBytes = length
		case opClose:
			return 0, io.EOF
		case opPing, opPong:
			if err := w.answerControl(opcode, length); err != nil {
				return 0, err
			}
		default:
			return 0, fmt.Errorf("the tunnel sent an unknown WebSocket frame type %#x", opcode)
		}
	}

	if uint64(len(p)) > w.unreadBytes {
		p = p[:w.unreadBytes]
	}
	n, err := w.reader.Read(p)
	w.unreadBytes -= uint64(n)
	return n, err
}

func (w *webSocket) readHeader() (byte, uint64, error) {
	var head [2]byte
	if _, err := io.ReadFull(w.reader, head[:]); err != nil {
		return 0, 0, err
	}
	if head[1]&maskBit != 0 {
		return 0, 0, fmt.Errorf("the tunnel sent a masked WebSocket frame")
	}

	opcode := head[0] & 0x0f
	switch head[1] {
	case 126:
		var length uint16
		err := binary.Read(w.reader, binary.BigEndian, &length)
		return opcode, uint64(length), err
	case 127:
		var length uint64
		err := binary.Read(w.reader, binary.BigEndian, &length)
		return opcode, length, err
	default:
		return opcode, uint64(head[1]), nil
	}
}

func (w *webSocket) answerControl(opcode byte, length uint64) error {
	if length > maxControlPayload {
		return fmt.Errorf("the tunnel sent an oversized WebSocket control frame")
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(w.reader, payload); err != nil {
		return err
	}
	if opcode != opPing {
		return nil
	}
	return w.writeFrame(opPong, payload)
}

func (w *webSocket) Write(p []byte) (int, error) {
	written := 0
	for written < len(p) {
		chunk := p[written:min(len(p), written+maxFramePayload)]
		if err := w.writeFrame(opBinary, chunk); err != nil {
			return written, err
		}
		written += len(chunk)
	}
	return written, nil
}

func (w *webSocket) writeFrame(opcode byte, payload []byte) error {
	frame := make([]byte, 0, len(payload)+14)
	frame = append(frame, finBit|opcode)
	switch {
	case len(payload) < 126:
		frame = append(frame, maskBit|byte(len(payload)))
	case len(payload) <= 0xffff:
		frame = append(frame, maskBit|126)
		frame = binary.BigEndian.AppendUint16(frame, uint16(len(payload)))
	default:
		frame = append(frame, maskBit|127)
		frame = binary.BigEndian.AppendUint64(frame, uint64(len(payload)))
	}

	var mask [4]byte
	rand.Read(mask[:])
	frame = append(frame, mask[:]...)
	for i, b := range payload {
		frame = append(frame, b^mask[i%4])
	}

	w.writeLock.Lock()
	defer w.writeLock.Unlock()
	_, err := w.conn.Write(frame)
	return err
}

func (w *webSocket) Close() error {
	return w.conn.Close()
}
