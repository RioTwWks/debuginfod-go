package storage

import (
	"strings"

	"github.com/your-username/debuginfod-go/pkg/elfcomment"
)

// UICommentInfo — секция .comment ELF для Web UI.
type UICommentInfo struct {
	Lines          []string `json:"lines,omitempty"`
	Toolchain      string   `json:"toolchain,omitempty"`
	Copyright      string   `json:"copyright,omitempty"`
	Labels         []string `json:"labels,omitempty"`
	ProductVersion string   `json:"product_version,omitempty"`
	GitCommit      string   `json:"git_commit,omitempty"`
}

// ArtifactDiskPath возвращает путь к ELF на диске или пустую строку (архив).
func ArtifactDiskPath(rec ArtifactRecord) string {
	if rec.ArchivePath != "" || rec.Archive != "" {
		return ""
	}
	if rec.FilePath != "" {
		return rec.FilePath
	}
	return strings.TrimSpace(rec.File)
}

// EnrichArtifactComment читает .comment с диска.
func EnrichArtifactComment(rec *ArtifactRecord) {
	if rec == nil {
		return
	}
	path := ArtifactDiskPath(*rec)
	if path == "" {
		return
	}
	info, err := elfcomment.InfoFromPath(path)
	if err != nil {
		return
	}
	rec.Comment = &UICommentInfo{
		Lines:          info.Lines,
		Toolchain:      info.Toolchain,
		Copyright:      info.Copyright,
		Labels:         info.Labels,
		ProductVersion: info.ProductVersion,
		GitCommit:      info.GitCommit,
	}
}
