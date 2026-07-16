package debugfilename

import (
	"testing"
)

func TestParse(t *testing.T) {
	info, err := Parse("lib.so.19.1.5.2899.debug")
	if err != nil {
		t.Fatal(err)
	}
	if info.Stem != "lib.so" || info.Version != "19.1.5" || info.BuildNum != 2899 {
		t.Fatalf("unexpected: %+v", info)
	}
}

func TestParseBuildDir(t *testing.T) {
	n, err := ParseBuildDir("build_482_2025-03-26_extra")
	if err != nil || n != 482 {
		t.Fatalf("got %d err=%v", n, err)
	}
}

func TestParseInvalid(t *testing.T) {
	if _, err := Parse("foo.debug"); err == nil {
		t.Fatal("expected error")
	}
}
