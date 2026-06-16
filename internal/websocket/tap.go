package websocket

import "io"

const maxFramePayload = 1 << 20 // 1 MiB safety cap per frame

// FrameInfo carries metadata about a reassembled WebSocket message.
type FrameInfo struct {
	Fragments int
}

// FrameHandler receives decoded WebSocket payloads after fragment reassembly.
type FrameHandler func(direction string, opcode byte, payload []byte, info FrameInfo)

type wireFrame struct {
	Fin     bool
	Opcode  byte
	Payload []byte
}

type parsedMessage struct {
	Opcode    byte
	Payload   []byte
	Fragments int
}

// FrameTap wraps a post-101 io.ReadWriter and observes RFC 6455 frames in transit.
type FrameTap struct {
	inner        io.ReadWriter
	onFrame      FrameHandler
	readRemain   []byte
	writeRemain  []byte
	readAssembly messageAssembler
	writeAssembly messageAssembler
}

func NewFrameTap(inner io.ReadWriter, onFrame FrameHandler) *FrameTap {
	return &FrameTap{inner: inner, onFrame: onFrame}
}

func (t *FrameTap) Read(p []byte) (int, error) {
	n, err := t.inner.Read(p)
	if n > 0 {
		t.observe(p[:n], false, "in", &t.readRemain, &t.readAssembly)
	}
	return n, err
}

func (t *FrameTap) Write(p []byte) (int, error) {
	if len(p) > 0 {
		t.observe(p, true, "out", &t.writeRemain, &t.writeAssembly)
	}
	return t.inner.Write(p)
}

func (t *FrameTap) Close() error {
	if c, ok := t.inner.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

func (t *FrameTap) observe(chunk []byte, masked bool, direction string, store *[]byte, assembler *messageAssembler) {
	*store = append(*store, chunk...)
	wireFrames, leftover := parseWireFrames(*store, masked)
	*store = leftover

	for _, frame := range wireFrames {
		if t.onFrame == nil {
			continue
		}

		if isControlOpcode(frame.Opcode) {
			if !frame.Fin {
				assembler.reset()
				continue
			}
			t.onFrame(direction, frame.Opcode, frame.Payload, FrameInfo{Fragments: 1})
			continue
		}

		message, ok := assembler.ingest(frame)
		if !ok {
			continue
		}
		t.onFrame(direction, message.Opcode, message.Payload, FrameInfo{Fragments: message.Fragments})
	}
}

type messageAssembler struct {
	active    bool
	opcode    byte
	buffer    []byte
	fragments int
}

func (a *messageAssembler) reset() {
	a.active = false
	a.opcode = 0
	a.buffer = a.buffer[:0]
	a.fragments = 0
}

func (a *messageAssembler) ingest(frame wireFrame) (*parsedMessage, bool) {
	if isControlOpcode(frame.Opcode) {
		return nil, false
	}

	if frame.Opcode != 0 {
		if a.active {
			a.reset()
		}
		a.active = true
		a.opcode = frame.Opcode
		a.fragments = 1
		a.buffer = append(a.buffer[:0], frame.Payload...)
	} else if a.active {
		a.fragments++
		a.buffer = append(a.buffer, frame.Payload...)
	} else {
		return nil, false
	}

	if len(a.buffer) > maxFramePayload {
		a.reset()
		return nil, false
	}

	if !frame.Fin {
		return nil, false
	}

	message := &parsedMessage{
		Opcode:    a.opcode,
		Payload:   append([]byte(nil), a.buffer...),
		Fragments: a.fragments,
	}
	a.reset()
	return message, true
}

func isControlOpcode(opcode byte) bool {
	return opcode == 0x8 || opcode == 0x9 || opcode == 0xA
}

func parseWireFrames(buf []byte, masked bool) ([]wireFrame, []byte) {
	var frames []wireFrame

	for len(buf) >= 2 {
		fin := (buf[0] & 0x80) != 0
		opcode := buf[0] & 0x0f
		maskBit := (buf[1] & 0x80) != 0
		payloadLen := int(buf[1] & 0x7f)
		offset := 2

		switch payloadLen {
		case 126:
			if len(buf) < 4 {
				return frames, buf
			}
			payloadLen = int(buf[2])<<8 | int(buf[3])
			offset = 4
		case 127:
			if len(buf) < 10 {
				return frames, buf
			}
			payloadLen = int(buf[6])<<24 | int(buf[7])<<16 | int(buf[8])<<8 | int(buf[9])
			offset = 10
		}

		if payloadLen < 0 || payloadLen > maxFramePayload {
			return frames, nil
		}

		var maskKey [4]byte
		if maskBit {
			if len(buf) < offset+4 {
				return frames, buf
			}
			copy(maskKey[:], buf[offset:offset+4])
			offset += 4
		} else if masked {
			return frames, buf
		}

		if len(buf) < offset+payloadLen {
			return frames, buf
		}

		payload := make([]byte, payloadLen)
		copy(payload, buf[offset:offset+payloadLen])
		if maskBit {
			for i := range payload {
				payload[i] ^= maskKey[i%4]
			}
		}

		frames = append(frames, wireFrame{
			Fin:     fin,
			Opcode:  opcode,
			Payload: payload,
		})
		buf = buf[offset+payloadLen:]
	}

	return frames, buf
}