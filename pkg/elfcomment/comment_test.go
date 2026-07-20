package elfcomment

import (
	"testing"
)

func TestParseBytesTagPrefix(t *testing.T) {
	data := []byte("tag: abc123def\x00")
	tag, err := ParseBytes(data)
	if err != nil || tag != "abc123def" {
		t.Fatalf("got %q err=%v", tag, err)
	}
}

func TestParseBytesIgnoresJIRA(t *testing.T) {
	data := []byte("GCC: (GNU) 11.2.0\x00DEVOPS-110\x00")
	_, err := ParseBytes(data)
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for JIRA-only comment, got %v", err)
	}
}

func TestParseBytesPrefersGitOverJIRA(t *testing.T) {
	data := []byte("DEVOPS-110\x00tag: deadbeef\x00")
	tag, err := ParseBytes(data)
	if err != nil || tag != "deadbeef" {
		t.Fatalf("got %q err=%v", tag, err)
	}
}

func TestParseBytesGitSemver(t *testing.T) {
	data := []byte("GCC: (GNU) 11.2.0\x00v16.0.0.10\x00")
	tag, err := ParseBytes(data)
	if err != nil || tag != "v16.0.0.10" {
		t.Fatalf("got %q err=%v", tag, err)
	}
}

func TestParseBytesCommitSHA(t *testing.T) {
	data := []byte("commit built from abcdef0123456789\x00")
	tag, err := ParseBytes(data)
	if err != nil || tag != "abcdef0123456789" {
		t.Fatalf("got %q err=%v", tag, err)
	}
}

func TestParseBytesNotFound(t *testing.T) {
	_, err := ParseBytes([]byte("GCC only\x00"))
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
