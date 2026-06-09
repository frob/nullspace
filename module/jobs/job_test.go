package jobs

import (
	"bytes"
	"testing"
)

// payloadStruct is a small struct used to round-trip through EncodePayload/DecodePayload.
type payloadStruct struct {
	Name  string   `json:"name"`
	Count int      `json:"count"`
	Tags  []string `json:"tags"`
}

func TestEncodePayload_Struct(t *testing.T) {
	t.Parallel()

	in := payloadStruct{
		Name:  "widget",
		Count: 42,
		Tags:  []string{"alpha", "beta"},
	}

	data, err := EncodePayload(in)
	if err != nil {
		t.Fatalf("EncodePayload: unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("EncodePayload: expected non-empty bytes for a struct")
	}

	var out payloadStruct
	if err := DecodePayload(data, &out); err != nil {
		t.Fatalf("DecodePayload: unexpected error: %v", err)
	}

	if out.Name != in.Name {
		t.Errorf("Name: got %q, want %q", out.Name, in.Name)
	}
	if out.Count != in.Count {
		t.Errorf("Count: got %d, want %d", out.Count, in.Count)
	}
	if len(out.Tags) != len(in.Tags) {
		t.Fatalf("Tags length: got %d, want %d", len(out.Tags), len(in.Tags))
	}
	for i := range in.Tags {
		if out.Tags[i] != in.Tags[i] {
			t.Errorf("Tags[%d]: got %q, want %q", i, out.Tags[i], in.Tags[i])
		}
	}
}

func TestEncodePayload_NilReturnsNil(t *testing.T) {
	t.Parallel()

	data, err := EncodePayload(nil)
	if err != nil {
		t.Fatalf("EncodePayload(nil): unexpected error: %v", err)
	}
	if data != nil {
		t.Fatalf("EncodePayload(nil): expected nil bytes, got %v", data)
	}
}

func TestEncodePayload_ByteSlicePassthrough(t *testing.T) {
	t.Parallel()

	raw := []byte("raw payload, not JSON-encoded twice")
	data, err := EncodePayload(raw)
	if err != nil {
		t.Fatalf("EncodePayload([]byte): unexpected error: %v", err)
	}
	if !bytes.Equal(data, raw) {
		t.Fatalf("EncodePayload([]byte): expected passthrough, got %q want %q", data, raw)
	}
}

func TestEncodePayload_UnencodableReturnsError(t *testing.T) {
	t.Parallel()

	// channels are not JSON-encodable
	ch := make(chan int)
	defer close(ch)

	_, err := EncodePayload(ch)
	if err == nil {
		t.Fatal("EncodePayload(chan int): expected non-nil error, got nil")
	}
}

func TestDecodePayload_InvalidJSON(t *testing.T) {
	t.Parallel()

	var out payloadStruct
	err := DecodePayload([]byte("{not valid json"), &out)
	if err == nil {
		t.Fatal("DecodePayload(invalid): expected non-nil error, got nil")
	}
}

func TestJobStatusValues(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		got  JobStatus
		want string
	}{
		{"StatusPending", StatusPending, "pending"},
		{"StatusLeased", StatusLeased, "leased"},
		{"StatusDone", StatusDone, "done"},
		{"StatusFailed", StatusFailed, "failed"},
		{"StatusCancelled", StatusCancelled, "cancelled"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if string(tc.got) != tc.want {
				t.Errorf("%s: got %q, want %q", tc.name, string(tc.got), tc.want)
			}
		})
	}
}
