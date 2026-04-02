package tcp

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

// Codec handles framing and deframing of TCP messages. Each message is a
// (command, payload) pair. The codec determines the wire format.
type Codec interface {
	// Decode reads the next message from the reader. It returns the command
	// name, the payload bytes, and any error. io.EOF indicates a clean close.
	Decode(r *bufio.Reader) (command string, payload []byte, err error)

	// Encode serializes a command and payload into a wire-format frame.
	Encode(command string, payload []byte) ([]byte, error)
}

// JSONLinesCodec uses newline-delimited JSON. Each message is a JSON object
// with a "command" field and an optional "payload" field:
//
//	{"command":"echo","payload":"aGVsbG8="}
//
// The payload is raw JSON (not base64) — it can be any JSON value.
type JSONLinesCodec struct{}

type jsonMessage struct {
	Command string          `json:"command"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

func (JSONLinesCodec) Decode(r *bufio.Reader) (string, []byte, error) {
	line, err := r.ReadBytes('\n')
	if err != nil {
		return "", nil, err
	}

	var msg jsonMessage
	if err := json.Unmarshal(line, &msg); err != nil {
		return "", nil, fmt.Errorf("tcp codec: invalid JSON: %w", err)
	}
	if msg.Command == "" {
		return "", nil, fmt.Errorf("tcp codec: missing command field")
	}

	return msg.Command, []byte(msg.Payload), nil
}

func (JSONLinesCodec) Encode(command string, payload []byte) ([]byte, error) {
	msg := jsonMessage{
		Command: command,
		Payload: json.RawMessage(payload),
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// LengthPrefixCodec uses a binary length-prefixed format:
//
//	[4 bytes: total frame length (big-endian)]
//	[2 bytes: command name length (big-endian)]
//	[N bytes: command name (UTF-8)]
//	[M bytes: payload]
//
// The total frame length covers command length + command + payload.
type LengthPrefixCodec struct{}

func (LengthPrefixCodec) Decode(r *bufio.Reader) (string, []byte, error) {
	// Read frame length.
	var frameLen uint32
	if err := binary.Read(r, binary.BigEndian, &frameLen); err != nil {
		return "", nil, err
	}

	if frameLen < 2 {
		return "", nil, fmt.Errorf("tcp codec: frame too short")
	}

	// Read command name length.
	var cmdLen uint16
	if err := binary.Read(r, binary.BigEndian, &cmdLen); err != nil {
		return "", nil, err
	}

	remaining := frameLen - 2
	if uint32(cmdLen) > remaining {
		return "", nil, fmt.Errorf("tcp codec: command length exceeds frame")
	}

	// Read command name.
	cmdBuf := make([]byte, cmdLen)
	if _, err := io.ReadFull(r, cmdBuf); err != nil {
		return "", nil, err
	}

	// Read payload.
	payloadLen := remaining - uint32(cmdLen)
	var payload []byte
	if payloadLen > 0 {
		payload = make([]byte, payloadLen)
		if _, err := io.ReadFull(r, payload); err != nil {
			return "", nil, err
		}
	}

	return string(cmdBuf), payload, nil
}

func (LengthPrefixCodec) Encode(command string, payload []byte) ([]byte, error) {
	cmdBytes := []byte(command)
	cmdLen := uint16(len(cmdBytes))
	frameLen := uint32(2 + len(cmdBytes) + len(payload))

	buf := make([]byte, 4+frameLen)
	binary.BigEndian.PutUint32(buf[0:4], frameLen)
	binary.BigEndian.PutUint16(buf[4:6], cmdLen)
	copy(buf[6:6+cmdLen], cmdBytes)
	copy(buf[6+cmdLen:], payload)

	return buf, nil
}

// CodecByName returns a codec by its configuration name.
func CodecByName(name string) (Codec, error) {
	switch name {
	case "json-lines", "json":
		return JSONLinesCodec{}, nil
	case "length-prefix", "binary":
		return LengthPrefixCodec{}, nil
	default:
		return nil, fmt.Errorf("tcp: unknown codec %q (available: json-lines, length-prefix)", name)
	}
}
