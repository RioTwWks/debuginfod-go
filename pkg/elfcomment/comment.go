package elfcomment

import (
	"debug/elf"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ErrNotFound — git-метка сборки не найдена в .comment.
var ErrNotFound = errors.New("build label not found in .comment")

// gitTagRe — git-теги/версии вида v1.2.3, v16.0.0.10, release-2026-04-17.
var gitTagRe = regexp.MustCompile(`(?:^|[\s:(])(v?\d+(?:\.\d+)+(?:[-+][\w.-]+)?|release[-_][\w.-]+)`)

// commitSHARe — hex commit id (short или full).
var commitSHARe = regexp.MustCompile(`\b[0-9a-f]{7,40}\b`)

// FromPath читает ELF и возвращает git-метку из .comment, если есть.
func FromPath(path string) (string, error) {
	f, err := elf.Open(path)
	if err != nil {
		return "", fmt.Errorf("open elf: %w", err)
	}
	defer f.Close()
	return FromELF(f)
}

// FromELF извлекает git-метку из открытого ELF (опционально).
func FromELF(f *elf.File) (string, error) {
	sec := f.Section(".comment")
	if sec == nil {
		return "", ErrNotFound
	}
	data, err := sec.Data()
	if err != nil {
		return "", fmt.Errorf("read .comment: %w", err)
	}
	return ParseBytes(data)
}

// ParseBytes ищет git-метку в .comment. JIRA-теги (DEVOPS-123) игнорируются.
func ParseBytes(data []byte) (string, error) {
	lines := splitCommentLines(data)

	for _, line := range lines {
		lower := strings.ToLower(line)
		for _, prefix := range []string{"tag:", "commit:", "build:", "git:"} {
			if strings.HasPrefix(lower, prefix) {
				val := strings.TrimSpace(line[len(prefix):])
				if val != "" {
					return val, nil
				}
			}
		}
	}

	for _, line := range lines {
		if isToolchainLine(line) {
			continue
		}
		if m := gitTagRe.FindStringSubmatch(line); len(m) > 1 {
			return strings.TrimSpace(m[1]), nil
		}
		if sha := commitSHARe.FindString(line); sha != "" {
			return sha, nil
		}
	}
	return "", ErrNotFound
}

func isToolchainLine(line string) bool {
	lower := strings.ToLower(line)
	for _, prefix := range []string{"gcc:", "clang:", "rustc:", "go version", "go build"} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func splitCommentLines(data []byte) []string {
	raw := strings.Split(string(data), "\x00")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// FromPathOrEmpty возвращает git-метку или пустую строку (для файлов без тега).
func FromPathOrEmpty(path string) string {
	tag, err := FromPath(path)
	if err != nil {
		return ""
	}
	return tag
}

// MustExist проверяет наличие файла перед чтением.
func MustExist(path string) (string, error) {
	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	return FromPath(path)
}
