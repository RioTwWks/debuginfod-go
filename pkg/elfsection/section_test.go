package elfsection

import (
	"debug/elf"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestExtractGNUBuildIDSection(t *testing.T) {
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc not available")
	}

	tmp := t.TempDir()
	src := filepath.Join(tmp, "main.c")
	bin := filepath.Join(tmp, "hello")
	if err := os.WriteFile(src, []byte("int main(){return 0;}"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("gcc", "-g", "-o", bin, src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("gcc: %v\n%s", err, out)
	}

	data, err := Extract(bin, ".note.gnu.build-id")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("empty section data")
	}

	f, err := elf.Open(bin)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if _, err := FromELF(f, ".note.gnu.build-id"); err != nil {
		t.Fatal(err)
	}
}

func TestExtractMissingSection(t *testing.T) {
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc not available")
	}

	tmp := t.TempDir()
	src := filepath.Join(tmp, "main.c")
	bin := filepath.Join(tmp, "hello")
	_ = os.WriteFile(src, []byte("int main(){return 0;}"), 0o644)
	_ = exec.Command("gcc", "-o", bin, src).Run()

	_, err := Extract(bin, ".nonexistent")
	if err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
