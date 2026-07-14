package pathsafe_test

import (
	"path/filepath"
	"testing"

	"github.com/your-username/debuginfod-go/internal/pathsafe"
)

func TestValidateHTTPSourcePath(t *testing.T) {
	tests := []struct {
		path string
		ok   bool
	}{
		{"/usr/src/hello.c", true},
		{"/usr/src/foo/bar.c", true},
		{"", false},
		{"relative.c", false},
		{"/usr/../etc/passwd", false},
		{"/usr/src/../../etc/passwd", false},
	}
	for _, tc := range tests {
		err := pathsafe.ValidateHTTPSourcePath(tc.path)
		if tc.ok && err != nil {
			t.Errorf("path %q: want ok, got %v", tc.path, err)
		}
		if !tc.ok && err == nil {
			t.Errorf("path %q: want error", tc.path)
		}
	}
}

func TestValidateSectionName(t *testing.T) {
	if err := pathsafe.ValidateSectionName(".note.gnu.build-id"); err != nil {
		t.Fatal(err)
	}
	if err := pathsafe.ValidateSectionName("../evil"); err == nil {
		t.Fatal("expected error for traversal")
	}
	if err := pathsafe.ValidateSectionName("foo/bar"); err == nil {
		t.Fatal("expected error for slash")
	}
}

func TestValidateMemberPath(t *testing.T) {
	if err := pathsafe.ValidateMemberPath("usr/bin/hello"); err != nil {
		t.Fatal(err)
	}
	if err := pathsafe.ValidateMemberPath("../etc/passwd"); err == nil {
		t.Fatal("expected traversal error")
	}
	if err := pathsafe.ValidateMemberPath("foo/../../etc/passwd"); err == nil {
		t.Fatal("expected traversal error")
	}
}

func TestAssertUnderRoots(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "bin", "hello")
	if err := pathsafe.AssertUnderRoots(inside, []string{root}); err != nil {
		t.Fatal(err)
	}
	outside := "/etc/passwd"
	if err := pathsafe.AssertUnderRoots(outside, []string{root}); err == nil {
		t.Fatal("expected outside root to fail")
	}
}

func TestAllowedRoots(t *testing.T) {
	cache := t.TempDir()
	scan := filepath.Join(t.TempDir(), "scan")
	roots := pathsafe.AllowedRoots([]string{scan}, cache)
	if len(roots) != 2 {
		t.Fatalf("roots=%v", roots)
	}
}
