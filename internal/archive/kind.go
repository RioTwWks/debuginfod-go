package archive

import (
	"path/filepath"
	"strings"
)

// Kind описывает тип архива или пакета.
type Kind int

const (
	KindNone Kind = iota
	KindDeb
	KindRPM
	KindAPK
	KindPacman
	KindTar
	KindSRPM
	KindDSC
)

// DetectKind определяет тип архива по имени файла.
func DetectKind(path string) Kind {
	lower := strings.ToLower(filepath.ToSlash(path))
	switch {
	case strings.HasSuffix(lower, ".deb"):
		return KindDeb
	case strings.HasSuffix(lower, ".src.rpm"), strings.HasSuffix(lower, ".srpm"):
		return KindSRPM
	case strings.HasSuffix(lower, ".rpm"):
		return KindRPM
	case strings.HasSuffix(lower, ".apk"):
		return KindAPK
	case strings.HasSuffix(lower, ".pacman"),
		strings.HasSuffix(lower, ".pkg.tar.zst"),
		strings.HasSuffix(lower, ".pkg.tar.gz"),
		strings.HasSuffix(lower, ".pkg.tar.xz"),
		strings.HasSuffix(lower, ".pkg.tar"):
		return KindPacman
	case strings.HasSuffix(lower, ".tar.zst"),
		strings.HasSuffix(lower, ".tar.gz"),
		strings.HasSuffix(lower, ".tar.xz"),
		strings.HasSuffix(lower, ".tgz"),
		strings.HasSuffix(lower, ".tar"):
		return KindTar
	case strings.HasSuffix(lower, ".dsc"):
		return KindDSC
	default:
		return KindNone
	}
}

// IsArchive возвращает true для пакетов с ELF-членами.
func IsArchive(path string) bool {
	switch DetectKind(path) {
	case KindDeb, KindRPM, KindAPK, KindPacman, KindTar:
		return true
	default:
		return false
	}
}

// IsSourcePackage возвращает true для SRPM и DSC.
func IsSourcePackage(path string) bool {
	switch DetectKind(path) {
	case KindSRPM, KindDSC:
		return true
	default:
		return false
	}
}

// tarSuffix возвращает суффикс сжатого tar по имени файла.
func tarSuffix(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".tar.zst"):
		return ".tar.zst"
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return ".tar.gz"
	case strings.HasSuffix(lower, ".tar.xz"):
		return ".tar.xz"
	case strings.HasSuffix(lower, ".tar"):
		return ".tar"
	default:
		return ""
	}
}
