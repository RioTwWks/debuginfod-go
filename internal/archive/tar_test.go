package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func buildTestELF(t *testing.T, tmp string) []byte {
	t.Helper()
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc not available")
	}
	src := filepath.Join(tmp, "main.c")
	bin := filepath.Join(tmp, "hello")
	_ = os.WriteFile(src, []byte("int main(){return 0;}"), 0o644)
	if out, err := exec.Command("gcc", "-g", "-o", bin, src).CombinedOutput(); err != nil {
		t.Fatalf("gcc: %v\n%s", err, out)
	}
	data, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeTarGzELF(t *testing.T, path, memberName string, elfData []byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{
		Name: memberName,
		Mode: 0o755,
		Size: int64(len(elfData)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(elfData); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeTarZstELF(t *testing.T, path, memberName string, elfData []byte) {
	t.Helper()
	var buf bytes.Buffer
	zw, err := zstd.NewWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	tw := tar.NewWriter(zw)
	if err := tw.WriteHeader(&tar.Header{
		Name: memberName,
		Mode: 0o755,
		Size: int64(len(elfData)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(elfData); err != nil {
		t.Fatal(err)
	}
	_ = tw.Close()
	_ = zw.Close()
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIsArchiveKinds(t *testing.T) {
	cases := map[string]bool{
		"pkg.deb":           true,
		"app.rpm":           true,
		"lib.apk":           true,
		"foo.pkg.tar.zst":   true,
		"symbols.tar.gz":    true,
		"debug.tar.zst":     true,
		"hello":             false,
		"pkg.src.rpm":       false,
		"pkg.dsc":           false,
	}
	for path, want := range cases {
		if got := IsArchive(path); got != want {
			t.Errorf("IsArchive(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestIsSourcePackage(t *testing.T) {
	if !IsSourcePackage("foo-1.0.src.rpm") {
		t.Fatal("expected src.rpm as source package")
	}
	if !IsSourcePackage("foo_1.0.dsc") {
		t.Fatal("expected dsc as source package")
	}
	if IsSourcePackage("foo.rpm") {
		t.Fatal("binary rpm is not source package")
	}
}

func TestListPlainTarGzELFMembers(t *testing.T) {
	tmp := t.TempDir()
	elf := buildTestELF(t, tmp)
	archivePath := filepath.Join(tmp, "debug.tar.gz")
	writeTarGzELF(t, archivePath, "usr/lib/debug/hello", elf)

	members, err := ListELFMembers(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 1 {
		t.Fatalf("members=%d, want 1", len(members))
	}
	if members[0].MemberPath != "usr/lib/debug/hello" {
		t.Fatalf("member=%q", members[0].MemberPath)
	}
}

func TestListAPKELFMembers(t *testing.T) {
	tmp := t.TempDir()
	elf := buildTestELF(t, tmp)
	archivePath := filepath.Join(tmp, "hello.apk")
	writeTarGzELF(t, archivePath, "usr/bin/hello", elf)

	members, err := ListELFMembers(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 1 {
		t.Fatalf("members=%d, want 1", len(members))
	}
}

func TestListPacmanELFMembers(t *testing.T) {
	tmp := t.TempDir()
	elf := buildTestELF(t, tmp)
	archivePath := filepath.Join(tmp, "hello.pkg.tar.zst")
	writeTarZstELF(t, archivePath, "usr/bin/hello", elf)

	members, err := ListELFMembers(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 1 {
		t.Fatalf("members=%d, want 1", len(members))
	}
}

func TestOpenMemberReaderPlainTar(t *testing.T) {
	tmp := t.TempDir()
	elf := buildTestELF(t, tmp)
	archivePath := filepath.Join(tmp, "debug.tar.gz")
	writeTarGzELF(t, archivePath, "usr/bin/hello", elf)

	rc, err := OpenMemberReader(archivePath, "usr/bin/hello")
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(elf) {
		t.Fatalf("size=%d, want %d", len(got), len(elf))
	}
}

func TestExtractMemberLazy(t *testing.T) {
	tmp := t.TempDir()
	elf := buildTestELF(t, tmp)
	archivePath := filepath.Join(tmp, "debug.tar.gz")
	writeTarGzELF(t, archivePath, "usr/bin/hello", elf)
	cacheDir := filepath.Join(tmp, "cache")

	path, err := ExtractMember(cacheDir, archivePath, "usr/bin/hello")
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil || st.Size() == 0 {
		t.Fatalf("extracted file missing: %v", err)
	}
}
