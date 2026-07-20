package elfcomment

import (
	"testing"
)

func TestParseBytesQuikComment(t *testing.T) {
	data := []byte("GCC: (AstraLinuxSE 8.3.0-6) 8.3.0\x00" +
		"(c) ARQA Technologies, 2000-2026\x00" +
		"DBSTUBRC Library\x00" +
		"Quik Server\x00" +
		"16.0.0.1\x00" +
		"9ae10425c6bbb99c7ee1f71a3941fd7aee058227\x00")
	tag, err := ParseBytes(data)
	if err != nil || tag != "9ae10425c6bbb99c7ee1f71a3941fd7aee058227" {
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

func TestParseBytesIgnoresJIRA(t *testing.T) {
	data := []byte("GCC: (GNU) 11.2.0\x00DEVOPS-110\x00")
	_, err := ParseBytes(data)
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for JIRA-only comment, got %v", err)
	}
}

func TestParseBytesPrefersSHAOverProductVersion(t *testing.T) {
	data := []byte("16.0.0.1\x009ae10425c6bbb99c7ee1f71a3941fd7aee058227\x00")
	tag, err := ParseBytes(data)
	if err != nil || tag != "9ae10425c6bbb99c7ee1f71a3941fd7aee058227" {
		t.Fatalf("got %q err=%v", tag, err)
	}
}

func TestParseBytesReleaseTag(t *testing.T) {
	data := []byte("release-2026-04-17\x00")
	tag, err := ParseBytes(data)
	if err != nil || tag != "release-2026-04-17" {
		t.Fatalf("got %q err=%v", tag, err)
	}
}

func TestParseBytesNotFound(t *testing.T) {
	_, err := ParseBytes([]byte("GCC only\x00Quik Server\x0016.0.0.1\x00"))
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
