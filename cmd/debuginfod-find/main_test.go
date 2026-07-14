package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveBaseURL(t *testing.T) {
	t.Setenv("DEBUGINFOD_URLS", "http://first:8002,http://second:8002")
	if got := resolveBaseURL(""); got != "http://first:8002" {
		t.Fatalf("got %q", got)
	}
	if got := resolveBaseURL("http://custom/"); got != "http://custom" {
		t.Fatalf("got %q", got)
	}
}

func TestDownload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ELF"))
	}))
	defer srv.Close()

	tmp := t.TempDir()
	out := filepath.Join(tmp, "bin")
	if err := download(srv.URL, out); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil || string(data) != "ELF" {
		t.Fatalf("data=%q err=%v", data, err)
	}
}

func TestDownloadNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.NotFound(w, nil)
	}))
	defer srv.Close()
	if err := download(srv.URL, ""); err == nil {
		t.Fatal("expected error")
	}
}
