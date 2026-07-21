package webapi

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/your-username/debuginfod-go/internal/archive"
	"github.com/your-username/debuginfod-go/internal/pathsafe"
	"github.com/your-username/debuginfod-go/internal/storage"
	"github.com/your-username/debuginfod-go/pkg/elfsection"
)

// DedupRestorer восстанавливает сжатые blob в кэш.
type DedupRestorer interface {
	RestoreToCache(cacheDir, filePath string) (string, error)
}

// resolveFilePathWithDedup учитывает zstd/CAS dedup и cache-aside.
func resolveFilePathWithDedup(cacheDir string, loc storage.ArtifactLocation, restorer DedupRestorer) (string, error) {
	if loc.FilePath != "" {
		if restorer != nil {
			return restorer.RestoreToCache(cacheDir, loc.FilePath)
		}
		return loc.FilePath, nil
	}
	if loc.ArchivePath == "" || loc.MemberPath == "" {
		return "", storage.ErrNotFound
	}
	if err := pathsafe.ValidateMemberPath(loc.MemberPath); err != nil {
		return "", err
	}
	return archive.ExtractMember(cacheDir, loc.ArchivePath, loc.MemberPath)
}

func resolveSourcePath(cacheDir string, loc storage.SourceLocation) (string, error) {
	if loc.FilePath != "" {
		return loc.FilePath, nil
	}
	if loc.ArchivePath == "" || loc.MemberPath == "" {
		return "", storage.ErrNotFound
	}
	if err := pathsafe.ValidateMemberPath(loc.MemberPath); err != nil {
		return "", err
	}
	return archive.ExtractMember(cacheDir, loc.ArchivePath, loc.MemberPath)
}

// openArtifact открывает ELF-файл для section API (с отложенным извлечением).
func openArtifact(cacheDir string, loc storage.ArtifactLocation, restorer DedupRestorer) (string, func(), error) {
	path, err := resolveFilePathWithDedup(cacheDir, loc, restorer)
	if err != nil {
		return "", nil, err
	}
	if loc.FilePath != "" {
		return path, func() {}, nil
	}
	return path, func() { _ = os.Remove(path) }, nil
}

// serveResolvedFile отдаёт файл клиенту с оригинальным именем.
func serveResolvedFile(w http.ResponseWriter, r *http.Request, path, downloadName string) {
	if downloadName == "" {
		downloadName = filepath.Base(path)
	}
	setContentDisposition(w, downloadName)
	http.ServeFile(w, r, path)
}

func setContentDisposition(w http.ResponseWriter, filename string) {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return
	}
	safe := strings.ReplaceAll(filename, `"`, "")
	w.Header().Set("Content-Disposition", `attachment; filename="`+safe+`"`)
}

// extractSectionFromLocations извлекает секцию ELF с поддержкой lazy-архивов.
func extractSectionFromLocations(cacheDir string, debuginfo, executable storage.ArtifactLocation, sectionName string, restorer DedupRestorer) ([]byte, error) {
	debugPath, cleanupDebug, err := openArtifactIfPresent(cacheDir, debuginfo, restorer)
	if err != nil {
		return nil, err
	}
	defer cleanupDebug()

	execPath, cleanupExec, err := openArtifactIfPresent(cacheDir, executable, restorer)
	if err != nil {
		return nil, err
	}
	defer cleanupExec()

	return elfsection.ExtractFirst(debugPath, execPath, sectionName)
}

func openArtifactIfPresent(cacheDir string, loc storage.ArtifactLocation, restorer DedupRestorer) (string, func(), error) {
	if loc.FilePath == "" && loc.ArchivePath == "" {
		return "", func() {}, nil
	}
	return openArtifact(cacheDir, loc, restorer)
}

// streamMember отдаёт содержимое члена архива без сохранения на диск (опционально).
func streamMember(w http.ResponseWriter, archivePath, memberPath string) error {
	if err := pathsafe.ValidateMemberPath(memberPath); err != nil {
		return err
	}
	rc, err := archive.OpenMemberReader(archivePath, memberPath)
	if err != nil {
		return err
	}
	defer rc.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	setContentDisposition(w, filepath.Base(memberPath))
	_, err = io.Copy(w, rc)
	return err
}

func logResolveError(op string, err error, attrs ...any) {
	args := append([]any{"err", err}, attrs...)
	slog.Error(op, args...)
}
