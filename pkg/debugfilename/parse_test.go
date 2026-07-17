package debugfilename

import (
	"testing"
)

func TestParseLibSo(t *testing.T) {
	info, err := Parse("lib.so.19.1.5.2899.debug")
	if err != nil {
		t.Fatal(err)
	}
	if info.Stem != "lib.so" || info.Version != "19.1.5" || info.BuildNum != 2899 {
		t.Fatalf("unexpected: %+v", info)
	}
}

func TestParseQuikHyphen(t *testing.T) {
	info, err := Parse("quik-16.0.0.10.debug")
	if err != nil {
		t.Fatal(err)
	}
	if info.Stem != "quik" || info.Version != "16.0.0" || info.BuildNum != 10 {
		t.Fatalf("unexpected: %+v", info)
	}
}

func TestParseQuikHyphenMultiPartStem(t *testing.T) {
	info, err := Parse("quik-server-16.0.0.10.debug")
	if err != nil {
		t.Fatal(err)
	}
	if info.Stem != "quik-server" || info.Version != "16.0.0" || info.BuildNum != 10 {
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
	cases := []string{
		"foo.debug",
		"quik-16.0.10.debug",
		"quik-16.0.0.debug",
	}
	for _, name := range cases {
		if _, err := Parse(name); err == nil {
			t.Fatalf("expected error for %q", name)
		}
	}
}
