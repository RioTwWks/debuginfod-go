package archive

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func FuzzListTarELFMembers(f *testing.F) {
	f.Add([]byte{0x1f, 0x8b, 0x08})
	f.Add([]byte("not a gzip stream"))

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = listTarELFMembersFromReader("/fake/bench.tar.gz", bytes.NewReader(data), ".tar.gz")
	})
}

func FuzzListDebELFMembers(f *testing.F) {
	f.Add([]byte("!<arch>\n"))
	f.Add([]byte("!<arch>\ndebian-binary  100644  0     0     100644  4         `\n"))
	f.Add([]byte("invalid header"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}
		tmp := t.TempDir()
		path := filepath.Join(tmp, "fuzz.deb")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return
		}
		_, _ = listDebELFMembers(path)
	})
}
