package cloudflare

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"io"
	"net"
	"testing"
)

func TestRouteOnlyAcceptsQuickTunnelHosts(t *testing.T) {
	tunnel := QuickTunnel{Executable: "/opt/my tools/sbx"}

	route, err := tunnel.Route("four-words-go-here.trycloudflare.com")
	if err != nil {
		t.Fatal(err)
	}
	if route.ProxyCommand != "'/opt/my tools/sbx' proxy cloudflare-quick %h" {
		t.Errorf("proxy command is %s", route.ProxyCommand)
	}

	for _, endpoint := range []string{
		"evil.example.com",
		"a.trycloudflare.com.evil.example",
		"x.trycloudflare.com; rm -rf ~",
		"-oProxyCommand=x.trycloudflare.com",
		"UPPER.trycloudflare.com",
		"",
	} {
		if _, err := tunnel.Route(endpoint); err == nil {
			t.Errorf("%q was accepted", endpoint)
		}
	}
}

func serverFrame(opcode byte, payload []byte) []byte {
	frame := []byte{finBit | opcode}
	switch {
	case len(payload) < 126:
		frame = append(frame, byte(len(payload)))
	case len(payload) <= 0xffff:
		frame = append(frame, 126)
		frame = binary.BigEndian.AppendUint16(frame, uint16(len(payload)))
	default:
		frame = append(frame, 127)
		frame = binary.BigEndian.AppendUint64(frame, uint64(len(payload)))
	}
	return append(frame, payload...)
}

func readClientFrame(t *testing.T, reader *bufio.Reader) (byte, []byte) {
	t.Helper()
	var head [2]byte
	if _, err := io.ReadFull(reader, head[:]); err != nil {
		t.Fatal(err)
	}
	if head[1]&maskBit == 0 {
		t.Fatal("the client sent an unmasked frame")
	}

	length := uint64(head[1] &^ maskBit)
	if length == 126 {
		var short uint16
		binary.Read(reader, binary.BigEndian, &short)
		length = uint64(short)
	}
	if length == 127 {
		binary.Read(reader, binary.BigEndian, &length)
	}

	var mask [4]byte
	io.ReadFull(reader, mask[:])
	payload := make([]byte, length)
	if _, err := io.ReadFull(reader, payload); err != nil {
		t.Fatal(err)
	}
	for i := range payload {
		payload[i] ^= mask[i%4]
	}
	return head[0] & 0x0f, payload
}

func TestWebSocketReadsDataAcrossFramesAndAnswersPings(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	socket := &webSocket{conn: client, reader: bufio.NewReader(client)}

	large := make([]byte, 70_000)
	rand.Read(large)
	pongs := make(chan []byte, 1)
	go func() {
		reader := bufio.NewReader(server)
		server.Write(serverFrame(opBinary, []byte("hello ")))
		server.Write(serverFrame(opPing, []byte("are you there")))
		_, pong := readClientFrame(t, reader)
		pongs <- pong
		server.Write(serverFrame(opBinary, nil))
		server.Write(serverFrame(opContinuation, []byte("world")))
		server.Write(serverFrame(opBinary, large))
		server.Write(serverFrame(opClose, nil))
	}()

	got, err := io.ReadAll(socket)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, append([]byte("hello world"), large...)) {
		t.Errorf("read %d bytes that do not match what the server sent", len(got))
	}
	if pong := <-pongs; string(pong) != "are you there" {
		t.Errorf("pong carried %q", pong)
	}
}

func TestWebSocketWritesMaskedBinaryFrames(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	socket := &webSocket{conn: client, reader: bufio.NewReader(client)}

	sent := make([]byte, 2*maxFramePayload+300)
	rand.Read(sent)
	go func() {
		socket.Write(sent)
	}()

	reader := bufio.NewReader(server)
	var received []byte
	for len(received) < len(sent) {
		opcode, payload := readClientFrame(t, reader)
		if opcode != opBinary {
			t.Fatalf("frame type %#x", opcode)
		}
		if len(payload) > maxFramePayload {
			t.Fatalf("frame of %d bytes", len(payload))
		}
		received = append(received, payload...)
	}
	if !bytes.Equal(received, sent) {
		t.Error("the frames do not carry what was written")
	}
}

func TestWebSocketRejectsMaskedServerFrames(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	socket := &webSocket{conn: client, reader: bufio.NewReader(client)}

	go server.Write([]byte{finBit | opBinary, maskBit | 1, 0, 0, 0, 0, 'x'})
	if _, err := socket.Read(make([]byte, 8)); err == nil {
		t.Error("a masked server frame was accepted")
	}
}
