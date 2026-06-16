package websocket

import (
	"bytes"
	"testing"
)

func TestParseWireFramesTextUnmasked(t *testing.T) {
	payload := []byte(`{"event":"ping"}`)
	frame := buildFrame(true, 0x1, payload, false)

	frames, remain := parseWireFrames(frame, false)
	if len(frames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(frames))
	}
	if len(remain) != 0 {
		t.Fatalf("expected no remainder, got %d bytes", len(remain))
	}
	if !frames[0].Fin || frames[0].Opcode != 0x1 {
		t.Fatalf("unexpected frame header: %+v", frames[0])
	}
	if string(frames[0].Payload) != string(payload) {
		t.Fatalf("unexpected payload: %s", frames[0].Payload)
	}
}

func TestParseWireFramesTextMasked(t *testing.T) {
	payload := []byte("hello")
	frame := buildFrame(true, 0x1, payload, true)

	frames, remain := parseWireFrames(frame, true)
	if len(frames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(frames))
	}
	if len(remain) != 0 {
		t.Fatalf("expected no remainder, got %d bytes", len(remain))
	}
	if string(frames[0].Payload) != "hello" {
		t.Fatalf("unexpected payload: %s", frames[0].Payload)
	}
}

func TestMessageAssemblerReassemblesFragments(t *testing.T) {
	var assembler messageAssembler

	first, ok := assembler.ingest(wireFrame{Fin: false, Opcode: 0x1, Payload: []byte(`{"event":`)})
	if ok || first != nil {
		t.Fatalf("expected incomplete first fragment to wait")
	}

	second, ok := assembler.ingest(wireFrame{Fin: true, Opcode: 0x0, Payload: []byte(`"tick"}`)})
	if !ok || second == nil {
		t.Fatalf("expected reassembled message")
	}
	if second.Fragments != 2 {
		t.Fatalf("expected 2 fragments, got %d", second.Fragments)
	}
	if string(second.Payload) != `{"event":"tick"}` {
		t.Fatalf("unexpected payload: %s", second.Payload)
	}
}

func TestFrameTapObservesFragmentsAndControlFrames(t *testing.T) {
	first := buildFrame(false, 0x1, []byte(`{"from":`), false)
	second := buildFrame(true, 0x0, []byte(`"server"}`), false)
	ping := buildFrame(true, 0x9, nil, false)

	var backend bytes.Buffer
	backend.Write(append(append([]byte{}, first...), second...))
	backend.Write(ping)

	var messages []string
	var controls []byte

	tap := NewFrameTap(&backend, func(direction string, opcode byte, payload []byte, info FrameInfo) {
		if opcode == 0x9 {
			controls = append(controls, payload...)
			return
		}
		if direction != "in" || opcode != 0x1 {
			t.Fatalf("unexpected frame: dir=%s opcode=%#x", direction, opcode)
		}
		if info.Fragments != 2 {
			t.Fatalf("expected 2 fragments, got %d", info.Fragments)
		}
		messages = append(messages, string(payload))
	})

	buf := make([]byte, 512)
	if _, err := tap.Read(buf); err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if len(messages) != 1 || messages[0] != `{"from":"server"}` {
		t.Fatalf("unexpected reassembled payload: %#v", messages)
	}
	if controls != nil {
		t.Fatalf("expected empty ping payload, got %#v", controls)
	}
}

func TestFrameTapObservesReadWrite(t *testing.T) {
	serverToClient := buildFrame(true, 0x1, []byte(`{"from":"server"}`), false)
	clientToServer := buildFrame(true, 0x1, []byte(`{"from":"client"}`), true)

	var backend bytes.Buffer
	backend.Write(serverToClient)

	tap := NewFrameTap(&backend, func(direction string, opcode byte, payload []byte, info FrameInfo) {
		switch direction {
		case "in":
			if string(payload) != `{"from":"server"}` {
				t.Fatalf("unexpected inbound payload: %s", payload)
			}
		case "out":
			if string(payload) != `{"from":"client"}` {
				t.Fatalf("unexpected outbound payload: %s", payload)
			}
		default:
			t.Fatalf("unexpected direction: %s", direction)
		}
		if opcode != 0x1 {
			t.Fatalf("expected text opcode, got %#x", opcode)
		}
		if info.Fragments != 1 {
			t.Fatalf("expected single fragment, got %d", info.Fragments)
		}
	})

	buf := make([]byte, 256)
	if _, err := tap.Read(buf); err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if _, err := tap.Write(clientToServer); err != nil {
		t.Fatalf("write failed: %v", err)
	}
}

func buildFrame(fin bool, opcode byte, payload []byte, masked bool) []byte {
	b0 := opcode
	if fin {
		b0 |= 0x80
	}
	header := []byte{b0}
	length := len(payload)

	var extended []byte
	switch {
	case length < 126:
		if masked {
			header = append(header, byte(length|0x80))
		} else {
			header = append(header, byte(length))
		}
	case length < 65536:
		if masked {
			header = append(header, 126|0x80)
		} else {
			header = append(header, 126)
		}
		extended = []byte{byte(length >> 8), byte(length)}
	default:
		tail := length
		if masked {
			header = append(header, 127|0x80)
		} else {
			header = append(header, 127)
		}
		extended = make([]byte, 8)
		for i := 7; i >= 0; i-- {
			extended[i] = byte(tail & 0xff)
			tail >>= 8
		}
	}

	frame := append(header, extended...)
	if masked {
		maskKey := []byte{0x12, 0x34, 0x56, 0x78}
		frame = append(frame, maskKey...)
		maskedPayload := make([]byte, len(payload))
		for i := range payload {
			maskedPayload[i] = payload[i] ^ maskKey[i%4]
		}
		frame = append(frame, maskedPayload...)
		return frame
	}

	frame = append(frame, payload...)
	return frame
}