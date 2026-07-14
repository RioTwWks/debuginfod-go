package archive

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func benchTarGzELFPath(b *testing.B) string {
	b.Helper()
	if _, err := exec.LookPath("gcc"); err != nil {
		b.Skip("gcc not available")
	}

	tmp := b.TempDir()
	elfData := buildTestELF(b, tmp)
	archivePath := filepath.Join(tmp, "bench.tar.gz")
	writeTarGzELF(b, archivePath, "usr/bin/hello", elfData)
	return archivePath
}

func BenchmarkListTarELFMembersFromFile(b *testing.B) {
	archivePath := benchTarGzELFPath(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = listTarELFMembersFromFile(archivePath)
	}
}

func BenchmarkDetectKind(b *testing.B) {
	paths := []string{
		"/usr/lib/debug/foo.deb",
		"/var/cache/dnf/foo.rpm",
		"/tmp/debug.tar.gz",
		"/tmp/debug.tar.zst",
		"/tmp/foo.src.rpm",
		"/tmp/hello.dsc",
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, p := range paths {
			_ = DetectKind(p)
		}
	}
}

func BenchmarkListDebELFMembersInvalid(b *testing.B) {
	tmp := b.TempDir()
	path := filepath.Join(tmp, "empty.deb")
	if err := os.WriteFile(path, []byte("!<arch>\n"), 0o644); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = listDebELFMembers(path)
	}
}
