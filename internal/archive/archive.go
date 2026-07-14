package archive

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Member описывает файл внутри архива.
type Member struct {
	ArchivePath string
	MemberPath  string
	Reader      func() (io.ReadCloser, error)
}

// ListELFMembers возвращает ELF-файлы внутри архива.
func ListELFMembers(archivePath string) ([]Member, error) {
	switch DetectKind(archivePath) {
	case KindDeb:
		return listDebELFMembers(archivePath)
	case KindRPM:
		return listRPMELFMembers(archivePath)
	case KindTar:
		return listTarELFMembersFromFile(archivePath)
	default:
		return nil, fmt.Errorf("unsupported archive: %s", archivePath)
	}
}

// ListSourceMembers возвращает исходные файлы из SRPM или DSC.
func ListSourceMembers(archivePath string) ([]Member, error) {
	switch DetectKind(archivePath) {
	case KindSRPM:
		return listSRPMSourceMembers(archivePath)
	case KindDSC:
		return listDSCSourceMembers(archivePath)
	default:
		return nil, fmt.Errorf("unsupported source package: %s", archivePath)
	}
}

// OpenMemberReader открывает поток члена архива для отложенного извлечения.
func OpenMemberReader(archivePath, memberPath string) (io.ReadCloser, error) {
	switch DetectKind(archivePath) {
	case KindDeb:
		return openDebMember(archivePath, memberPath)
	case KindRPM:
		return openRPMMember(archivePath, memberPath)
	case KindTar:
		suffix := tarSuffix(archivePath)
		if suffix == "" {
			return nil, fmt.Errorf("unsupported tar archive: %s", archivePath)
		}
		return openTarArchiveMember(archivePath, memberPath, suffix)
	case KindSRPM:
		if strings.Contains(memberPath, "!") {
			return openNestedMember(archivePath, memberPath)
		}
		return openSRPMMember(archivePath, memberPath)
	case KindDSC:
		return openDSCMember(archivePath, memberPath)
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

// ExtractMember извлекает член архива по пути (для HTTP-запросов).
func ExtractMember(cacheDir, archivePath, memberPath string) (string, error) {
	return ExtractToCache(cacheDir, archivePath, memberPath, func() (io.ReadCloser, error) {
		return OpenMemberReader(archivePath, memberPath)
	})
}

func cacheKey(archivePath, memberPath string) string {
	base := strings.NewReplacer(string(os.PathSeparator), "_", "/", "_", ":", "_", "!", "_").
		Replace(archivePath + "!" + memberPath)
	return base
}
