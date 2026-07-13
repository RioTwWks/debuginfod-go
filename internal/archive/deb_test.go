package archive

import (
	"archive/tar"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func TestOpenCompressedTarZst(t *testing.T) {
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc not available")
	}

	tmp := t.TempDir()
	src := filepath.Join(tmp, "main.c")
	bin := filepath.Join(tmp, "hello")
	_ = os.WriteFile(src, []byte("int main(){return 0;}"), 0o644)
	_ = exec.Command("gcc", "-g", "-o", bin, src).Run()

	elfData, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}

	var tarBuf bytes.Buffer
	zw, err := zstd.NewWriter(&tarBuf)
	if err != nil {
		t.Fatal(err)
	}
	tw := tar.NewWriter(zw)
	if err := tw.WriteHeader(&tar.Header{
		Name: "usr/bin/hello",
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
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	rc, err := openCompressedTar(bytes.NewReader(tarBuf.Bytes()), "data.tar.zst")
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()

	members, err := listTarELFMembers("/fake/pkg.deb", tarBuf.Bytes(), "data.tar.zst")
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 1 {
		t.Fatalf("members=%d, want 1", len(members))
	}
}
