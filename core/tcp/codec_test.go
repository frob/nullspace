package tcp

import (
	"bufio"
	"bytes"
	"io"
	"testing"
)

func TestJSONLinesCodecRoundTrip(t *testing.T) {
	codec := JSONLinesCodec{}

	encoded, err := codec.Encode("echo", []byte(`"hello"`))
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	r := bufio.NewReader(bytes.NewReader(encoded))
	cmd, payload, err := codec.Decode(r)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if cmd != "echo" {
		t.Errorf("command = %q, want %q", cmd, "echo")
	}
	if string(payload) != `"hello"` {
		t.Errorf("payload = %q, want %q", string(payload), `"hello"`)
	}
}

func TestJSONLinesCodecNilPayload(t *testing.T) {
	codec := JSONLinesCodec{}

	encoded, err := codec.Encode("ping", nil)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	r := bufio.NewReader(bytes.NewReader(encoded))
	cmd, payload, err := codec.Decode(r)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if cmd != "ping" {
		t.Errorf("command = %q, want %q", cmd, "ping")
	}
	if len(payload) != 0 {
		t.Errorf("payload = %q, want empty", string(payload))
	}
}

func TestJSONLinesCodecMissingCommand(t *testing.T) {
	codec := JSONLinesCodec{}
	r := bufio.NewReader(bytes.NewReader([]byte("{\"payload\":\"test\"}\n")))

	_, _, err := codec.Decode(r)
	if err == nil {
		t.Fatal("expected error for missing command")
	}
}

func TestJSONLinesCodecMultipleMessages(t *testing.T) {
	codec := JSONLinesCodec{}

	var buf bytes.Buffer
	for _, cmd := range []string{"a", "b", "c"} {
		encoded, _ := codec.Encode(cmd, []byte(`"x"`))
		buf.Write(encoded)
	}

	r := bufio.NewReader(&buf)
	for _, want := range []string{"a", "b", "c"} {
		cmd, _, err := codec.Decode(r)
		if err != nil {
			t.Fatalf("decode %q: %v", want, err)
		}
		if cmd != want {
			t.Errorf("command = %q, want %q", cmd, want)
		}
	}

	// Next read should be EOF.
	_, _, err := codec.Decode(r)
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}
}

func TestLengthPrefixCodecRoundTrip(t *testing.T) {
	codec := LengthPrefixCodec{}

	encoded, err := codec.Encode("echo", []byte("hello"))
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	r := bufio.NewReader(bytes.NewReader(encoded))
	cmd, payload, err := codec.Decode(r)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if cmd != "echo" {
		t.Errorf("command = %q, want %q", cmd, "echo")
	}
	if string(payload) != "hello" {
		t.Errorf("payload = %q, want %q", string(payload), "hello")
	}
}

func TestLengthPrefixCodecEmptyPayload(t *testing.T) {
	codec := LengthPrefixCodec{}

	encoded, err := codec.Encode("ping", nil)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	r := bufio.NewReader(bytes.NewReader(encoded))
	cmd, payload, err := codec.Decode(r)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if cmd != "ping" {
		t.Errorf("command = %q, want %q", cmd, "ping")
	}
	if len(payload) != 0 {
		t.Errorf("payload = %v, want empty", payload)
	}
}

func TestLengthPrefixCodecMultipleMessages(t *testing.T) {
	codec := LengthPrefixCodec{}

	var buf bytes.Buffer
	for _, cmd := range []string{"a", "b", "c"} {
		encoded, _ := codec.Encode(cmd, []byte("data"))
		buf.Write(encoded)
	}

	r := bufio.NewReader(&buf)
	for _, want := range []string{"a", "b", "c"} {
		cmd, payload, err := codec.Decode(r)
		if err != nil {
			t.Fatalf("decode %q: %v", want, err)
		}
		if cmd != want {
			t.Errorf("command = %q, want %q", cmd, want)
		}
		if string(payload) != "data" {
			t.Errorf("payload = %q, want %q", string(payload), "data")
		}
	}
}

func TestCodecByName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"json-lines", false},
		{"json", false},
		{"length-prefix", false},
		{"binary", false},
		{"unknown", true},
	}

	for _, tt := range tests {
		codec, err := CodecByName(tt.name)
		if tt.wantErr {
			if err == nil {
				t.Errorf("CodecByName(%q) expected error", tt.name)
			}
		} else {
			if err != nil {
				t.Errorf("CodecByName(%q) = %v", tt.name, err)
			}
			if codec == nil {
				t.Errorf("CodecByName(%q) returned nil codec", tt.name)
			}
		}
	}
}
