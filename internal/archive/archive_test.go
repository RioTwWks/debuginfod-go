package archive

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestListRPMELFMembers(t *testing.T) {
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc not available")
	}
	if _, err := exec.LookPath("rpmbuild"); err != nil {
		t.Skip("rpmbuild not available")
	}

	tmp := t.TempDir()
	src := filepath.Join(tmp, "main.c")
	bin := filepath.Join(tmp, "usr/bin/hello")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("int main(){return 0;}"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("gcc", "-g", "-o", bin, src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("gcc: %v\n%s", err, out)
	}

	spec := filepath.Join(tmp, "hello.spec")
	if err := os.WriteFile(spec, []byte(`
Name: hello
Version: 1.0
Release: 1
Summary: test
License: MIT
%description
test
%files
/usr/bin/hello
`), 0o644); err != nil {
		t.Fatal(err)
	}

	rpmDir := filepath.Join(tmp, "rpmbuild")
	for _, sub := range []string{"BUILD", "RPMS", "SOURCES", "SPECS", "SRPMS"} {
		if err := os.MkdirAll(filepath.Join(rpmDir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	build := exec.Command("rpmbuild", "-bb", spec,
		"--define", "_topdir "+rpmDir,
		"--define", "_builddir "+tmp,
	)
	build.Dir = tmp
	if out, err := build.CombinedOutput(); err != nil {
		t.Skipf("rpmbuild unavailable or failed: %v\n%s", err, out)
	}

	matches, err := filepath.Glob(filepath.Join(rpmDir, "RPMS", "*", "hello-*.rpm"))
	if err != nil || len(matches) == 0 {
		t.Skip("rpm package not built")
	}

	members, err := ListELFMembers(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(members) == 0 {
		t.Fatal("expected ELF members in rpm")
	}
}
