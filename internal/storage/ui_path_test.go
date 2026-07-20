package storage

import "testing"

func TestRelativeToScanRoots(t *testing.T) {
	roots := []string{"/data/debug_linux"}
	got := RelativeToScanRoots("/data/debug_linux/Released/Quik/build_1/lib.debug", roots)
	want := "Released/Quik/build_1/lib.debug"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}

	got = RelativeToScanRoots("/other/path/file.debug", roots)
	if got != "/other/path/file.debug" {
		t.Fatalf("fallback got %q", got)
	}
}

func TestArtifactDisplayPathArchive(t *testing.T) {
	roots := []string{"/scan"}
	rec := ArtifactRecord{
		Archive: "/scan/pkg.tar",
		File:    "usr/lib/debug.so",
	}
	got := ArtifactDisplayPath(rec, roots)
	if got != "pkg.tar → usr/lib/debug.so" {
		t.Fatalf("got %q", got)
	}
}
