package storage

import (
	"path/filepath"
	"strings"
	"time"
)

// RelativeToScanRoots возвращает путь относительно ближайшего scan root.
// Если префикс не найден — исходный путь в slash-форме.
func RelativeToScanRoots(absPath string, scanRoots []string) string {
	absPath = filepath.ToSlash(filepath.Clean(absPath))
	if absPath == "" {
		return ""
	}

	best := ""
	for _, root := range scanRoots {
		rootAbs, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		rootAbs = filepath.ToSlash(filepath.Clean(rootAbs))
		rootAbs = strings.TrimSuffix(rootAbs, "/")
		if absPath == rootAbs {
			return ""
		}
		prefix := rootAbs + "/"
		if !strings.HasPrefix(absPath, prefix) {
			continue
		}
		rel := strings.TrimPrefix(absPath, prefix)
		if best == "" || len(rel) < len(best) {
			best = rel
		}
	}
	if best != "" {
		return best
	}
	return absPath
}

// ArtifactDisplayPath — относительный путь для UI (архив → member).
func ArtifactDisplayPath(rec ArtifactRecord, scanRoots []string) string {
	if rec.Archive != "" {
		archRel := RelativeToScanRoots(rec.Archive, scanRoots)
		if archRel != rec.Archive {
			return archRel + " → " + rec.File
		}
		return rec.Archive + " → " + rec.File
	}
	return RelativeToScanRoots(rec.File, scanRoots)
}

// ArtifactFilename возвращает имя файла для отображения.
func ArtifactFilename(rec ArtifactRecord) string {
	return filepath.Base(rec.File)
}

// ArtifactDirectory возвращает каталог относительно scan root.
func ArtifactDirectory(rec ArtifactRecord, scanRoots []string) string {
	rel := ArtifactDisplayPath(rec, scanRoots)
	if i := strings.LastIndex(rel, "/"); i >= 0 {
		return rel[:i]
	}
	return ""
}

// EnrichArtifactRecord дополняет запись полями для Web UI.
func EnrichArtifactRecord(rec *ArtifactRecord, scanRoots []string) {
	if rec == nil {
		return
	}
	if rec.File != "" {
		rec.RelativePath = ArtifactDisplayPath(*rec, scanRoots)
		rec.Filename = ArtifactFilename(*rec)
		rec.Directory = ArtifactDirectory(*rec, scanRoots)
	} else if rec.RelativePath != "" {
		if rec.Filename == "" {
			rec.Filename = filepath.Base(rec.RelativePath)
		}
		if rec.Directory == "" {
			if i := strings.LastIndex(rec.RelativePath, "/"); i >= 0 {
				rec.Directory = rec.RelativePath[:i]
			}
		}
	}
	if rec.ArchivePath != "" {
		rec.ArchiveRel = RelativeToScanRoots(rec.ArchivePath, scanRoots)
	}
	if rec.MtimeNs > 0 {
		rec.Mtime = time.Unix(0, rec.MtimeNs).UTC().Format(time.RFC3339)
	}
}
