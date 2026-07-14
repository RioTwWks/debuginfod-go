package archive

import "testing"

func TestDetectKindTargetOS(t *testing.T) {
	cases := map[string]Kind{
		"pkg.deb":         KindDeb,
		"app.rpm":         KindRPM,
		"foo.src.rpm":     KindSRPM,
		"pkg.dsc":         KindDSC,
		"symbols.tar.gz":  KindTar,
		"debug.tar.zst":   KindTar,
		"hello":           KindNone,
		"lib.apk":         KindNone,
		"foo.pacman":      KindNone,
		"bar.pkg.tar.zst": KindNone,
	}
	for path, want := range cases {
		if got := DetectKind(path); got != want {
			t.Errorf("DetectKind(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestIsArchiveKinds(t *testing.T) {
	cases := map[string]bool{
		"pkg.deb":           true,
		"app.rpm":           true,
		"symbols.tar.gz":    true,
		"debug.tar.zst":     true,
		"hello":             false,
		"pkg.src.rpm":       false,
		"pkg.dsc":           false,
		"lib.apk":           false,
		"foo.pkg.tar.zst":   false,
		"bar.pacman":        false,
	}
	for path, want := range cases {
		if got := IsArchive(path); got != want {
			t.Errorf("IsArchive(%q) = %v, want %v", path, got, want)
		}
	}
}
