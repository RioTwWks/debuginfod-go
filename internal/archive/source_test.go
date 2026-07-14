package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestListSRPMSourceMembersFromTarball(t *testing.T) {
	srcContent := []byte("int main() { return 0; }\n")
	var tarBuf bytes.Buffer
	gz := gzip.NewWriter(&tarBuf)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{Name: "hello-1.0/main.c", Mode: 0o644, Size: int64(len(srcContent))})
	_, _ = tw.Write(srcContent)
	_ = tw.Close()
	_ = gz.Close()

	members, err := listTarSourceMembersFromReader("/fake/src.rpm", bytes.NewReader(tarBuf.Bytes()), ".tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 1 {
		t.Fatalf("members=%d, want 1", len(members))
	}
	if members[0].MemberPath != "hello-1.0/main.c" {
		t.Fatalf("member=%q", members[0].MemberPath)
	}
}

func TestParseDSCFiles(t *testing.T) {
	tmp := t.TempDir()
	dsc := filepath.Join(tmp, "hello_1.0.dsc")
	content := `Format: 3.0 (quilt)
Source: hello
Files:
 1234 hello_1.0.orig.tar.gz
 5678 hello_1.0.debian.tar.xz
`
	if err := os.WriteFile(dsc, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := parseDSCFiles(dsc)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("files=%v", files)
	}
	if files[0] != "hello_1.0.orig.tar.gz" {
		t.Fatalf("first file=%q", files[0])
	}
}
