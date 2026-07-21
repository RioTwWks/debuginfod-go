package elfcomment

import (
	"debug/elf"
	"fmt"
	"strings"
)

// CommentInfo — разобранная секция .comment для UI/inspect.
type CommentInfo struct {
	Lines          []string `json:"lines"`
	Toolchain      string   `json:"toolchain,omitempty"`
	Copyright      string   `json:"copyright,omitempty"`
	Labels         []string `json:"labels,omitempty"`
	ProductVersion string   `json:"product_version,omitempty"`
	GitCommit      string   `json:"git_commit,omitempty"`
}

// ParseInfo разбирает .comment на поля для отображения.
func ParseInfo(data []byte) CommentInfo {
	lines := splitCommentLines(data)
	info := CommentInfo{Lines: append([]string(nil), lines...)}

	if commit, err := ParseBytes(data); err == nil {
		info.GitCommit = commit
	}

	for _, line := range lines {
		lower := strings.ToLower(line)
		if isToolchainLine(line) {
			if info.Toolchain == "" {
				info.Toolchain = line
			}
			continue
		}
		if strings.HasPrefix(lower, "(c)") {
			info.Copyright = line
			continue
		}
		if productVersionRe.MatchString(line) {
			info.ProductVersion = line
			continue
		}
		if fullCommitLineRe.MatchString(line) || shortCommitLineRe.MatchString(line) {
			continue
		}
		if prefixedGitLabel(line) {
			continue
		}
		info.Labels = append(info.Labels, line)
	}
	return info
}

func prefixedGitLabel(line string) bool {
	lower := strings.ToLower(line)
	for _, prefix := range []string{"tag:", "commit:", "build:", "git:"} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// InfoFromPath читает и разбирает .comment из ELF на диске.
func InfoFromPath(path string) (CommentInfo, error) {
	f, err := elf.Open(path)
	if err != nil {
		return CommentInfo{}, fmt.Errorf("open elf: %w", err)
	}
	defer f.Close()
	return InfoFromELF(f)
}

// InfoFromELF разбирает .comment из открытого ELF.
func InfoFromELF(f *elf.File) (CommentInfo, error) {
	sec := f.Section(".comment")
	if sec == nil {
		return CommentInfo{}, ErrNotFound
	}
	data, err := sec.Data()
	if err != nil {
		return CommentInfo{}, fmt.Errorf("read .comment: %w", err)
	}
	info := ParseInfo(data)
	if len(info.Lines) == 0 {
		return CommentInfo{}, ErrNotFound
	}
	return info, nil
}
