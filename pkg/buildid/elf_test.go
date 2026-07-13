package buildid

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFromPathWithCompiledBinary(t *testing.T) {
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc not available")
	}

	tmp := t.TempDir()
	src := filepath.Join(tmp, "main.c")
	bin := filepath.Join(tmp, "testbin")

	if err := os.WriteFile(src, []byte("int main(){return 0;}"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("gcc", "-g", "-o", bin, src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("gcc failed: %v\n%s", err, out)
	}

	id, err := FromPath(bin)
	if err != nil {
		t.Fatalf("FromPath: %v", err)
	}
	if len(id) != 40 {
		t.Fatalf("unexpected build-id length: %q", id)
	}

	artifact, err := OpenELF(bin)
	if err != nil {
		t.Fatal(err)
	}
	defer artifact.Close()

	if got := ArtifactType(bin, artifact); got != "executable" {
		t.Fatalf("ArtifactType = %q, want executable", got)
	}
}

func TestNormalize(t *testing.T) {
	if got := Normalize("AB-CD"); got != "abcd" {
		t.Fatalf("Normalize = %q, want abcd", got)
	}
}
