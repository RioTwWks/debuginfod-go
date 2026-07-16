package elfcomment

import (
	"testing"
)

func TestParseBytesDEVOPS(t *testing.T) {
	data := []byte("GCC: (GNU) 11.2.0\x00DEVOPS-110\x00")
	tag, err := ParseBytes(data)
	if err != nil || tag != "DEVOPS-110" {
		t.Fatalf("got %q err=%v", tag, err)
	}
}

func TestParseBytesTagPrefix(t *testing.T) {
	data := []byte("tag: abc123def\x00")
	tag, err := ParseBytes(data)
	if err != nil || tag != "abc123def" {
		t.Fatalf("got %q err=%v", tag, err)
	}
}

func TestParseBytesNotFound(t *testing.T) {
	_, err := ParseBytes([]byte("GCC only\x00"))
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
