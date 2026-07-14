// Package pathsafe проверяет пути на path traversal и выход за разрешённые корни.
package pathsafe

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrUnsafePath возвращается при недопустимом пути.
var ErrUnsafePath = errors.New("unsafe path")

// ValidateHTTPSourcePath проверяет путь из URL /buildid/.../source/<path>.
func ValidateHTTPSourcePath(path string) error {
	if path == "" {
		return fmt.Errorf("%w: empty source path", ErrUnsafePath)
	}
	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("%w: source path must be absolute", ErrUnsafePath)
	}
	return validateNoTraversal(path)
}

// ValidateSectionName проверяет имя ELF-секции из URL.
func ValidateSectionName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: empty section name", ErrUnsafePath)
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("%w: section name must not contain separators", ErrUnsafePath)
	}
	return validateNoTraversal(name)
}

// ValidateMemberPath проверяет путь члена внутри архива.
func ValidateMemberPath(memberPath string) error {
	if memberPath == "" {
		return fmt.Errorf("%w: empty member path", ErrUnsafePath)
	}
	norm := filepath.ToSlash(filepath.Clean(memberPath))
	if strings.HasPrefix(norm, "/") || strings.HasPrefix(norm, "../") || strings.Contains(norm, "/../") {
		return fmt.Errorf("%w: member path escapes archive root", ErrUnsafePath)
	}
	return validateNoTraversal(memberPath)
}

// ValidateArchivePath проверяет путь к архиву на диске перед чтением.
func ValidateArchivePath(archivePath string, allowedRoots []string) error {
	if archivePath == "" {
		return fmt.Errorf("%w: empty archive path", ErrUnsafePath)
	}
	return AssertUnderRoots(archivePath, allowedRoots)
}

// AssertUnderRoots проверяет, что путь находится под одним из корней (после Clean/Abs).
func AssertUnderRoots(path string, roots []string) error {
	if len(roots) == 0 {
		return nil
	}
	absPath, err := absClean(path)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnsafePath, err)
	}
	for _, root := range roots {
		absRoot, err := absClean(root)
		if err != nil {
			continue
		}
		if underRoot(absPath, absRoot) {
			return nil
		}
	}
	return fmt.Errorf("%w: %s is outside allowed roots", ErrUnsafePath, path)
}

func validateNoTraversal(path string) error {
	if strings.Contains(path, "\x00") {
		return fmt.Errorf("%w: null byte in path", ErrUnsafePath)
	}
	slash := filepath.ToSlash(path)
	for _, p := range strings.Split(slash, "/") {
		if p == ".." {
			return fmt.Errorf("%w: path traversal", ErrUnsafePath)
		}
	}
	return nil
}

func absClean(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty path")
	}
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	// Разрешаем symlinks только для существующих путей; иначе Clean+Abs достаточно.
	if st, err := os.Lstat(abs); err == nil && st.Mode()&os.ModeSymlink != 0 {
		resolved, err := filepath.EvalSymlinks(abs)
		if err != nil {
			return "", err
		}
		abs = resolved
	}
	return abs, nil
}

func underRoot(path, root string) bool {
	if path == root {
		return true
	}
	sep := string(os.PathSeparator)
	return strings.HasPrefix(path, root+sep)
}

// AllowedRoots объединяет scan paths и cache dir для проверки отдачи файлов.
func AllowedRoots(scanPaths []string, cacheDir string) []string {
	seen := make(map[string]struct{})
	var roots []string
	add := func(p string) {
		if p == "" {
			return
		}
		abs, err := absClean(p)
		if err != nil {
			abs = filepath.Clean(p)
		}
		if _, ok := seen[abs]; ok {
			return
		}
		seen[abs] = struct{}{}
		roots = append(roots, abs)
	}
	for _, p := range scanPaths {
		add(p)
	}
	add(cacheDir)
	return roots
}
