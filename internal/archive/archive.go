package archive

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Member описывает файл внутри .deb или .rpm архива.
type Member struct {
	ArchivePath string
	MemberPath  string
	Reader      func() (io.ReadCloser, error)
}

// IsArchive возвращает true для поддерживаемых пакетов.
func IsArchive(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".deb" || ext == ".rpm"
}

// ListELFMembers возвращает ELF-файлы внутри архива.
func ListELFMembers(archivePath string) ([]Member, error) {
	ext := strings.ToLower(filepath.Ext(archivePath))
	switch ext {
	case ".deb":
		return listDebELFMembers(archivePath)
	case ".rpm":
		return listRPMELFMembers(archivePath)
	default:
		return nil, fmt.Errorf("unsupported archive: %s", archivePath)
	}
}

// ExtractToCache извлекает член архива в каталог кэша и возвращает путь.
func ExtractToCache(cacheDir, archivePath, memberPath string, open func() (io.ReadCloser, error)) (string, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", err
	}

	key := cacheKey(archivePath, memberPath)
	dest := filepath.Join(cacheDir, key)
	if st, err := os.Stat(dest); err == nil && !st.IsDir() {
		return dest, nil
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}

	rc, err := open()
	if err != nil {
		return "", err
	}
	defer rc.Close()

	f, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, rc); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return dest, nil
}

func cacheKey(archivePath, memberPath string) string {
	base := strings.NewReplacer(string(os.PathSeparator), "_", "/", "_", ":", "_").
		Replace(archivePath + "!" + memberPath)
	return base
}
