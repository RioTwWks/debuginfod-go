package buildid

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFromPathWithGCCBinary(t *testing.T) {
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

	result, err := FromPathDetailed(bin)
	if err != nil {
		t.Fatalf("FromPathDetailed: %v", err)
	}
	if result.Kind != KindGNU {
		t.Fatalf("Kind = %q, want gnu", result.Kind)
	}
	if len(result.Value) != 40 {
		t.Fatalf("unexpected build-id length: %q", result.Value)
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

func TestFromPathWithGoBinary(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "main.go")
	bin := filepath.Join(tmp, "testbin")

	if err := os.WriteFile(src, []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("go", "build", "-o", bin, src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	result, err := FromPathDetailed(bin)
	if err != nil {
		t.Fatalf("FromPathDetailed: %v", err)
	}
	if result.Kind != KindGo {
		t.Fatalf("Kind = %q, want go", result.Kind)
	}
	if result.Raw == "" {
		t.Fatal("expected raw Go build-id")
	}
	if len(result.Value) != 64 {
		t.Fatalf("Go canonical id length = %d, want 64", len(result.Value))
	}
}

func TestNormalize(t *testing.T) {
	if got := Normalize("AB-CD"); got != "abcd" {
		t.Fatalf("Normalize = %q, want abcd", got)
	}
}

func TestGoCanonicalIDStable(t *testing.T) {
	raw := "test/action/sum"
	a := GoCanonicalID(raw)
	b := GoCanonicalID(raw)
	if a != b {
		t.Fatalf("canonical id not stable: %q vs %q", a, b)
	}
}
