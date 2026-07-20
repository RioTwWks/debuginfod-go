package elfcomment

import (
	"debug/elf"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ErrNotFound — git commit не найден в .comment.
var ErrNotFound = errors.New("build label not found in .comment")

// fullCommitLineRe — строка .comment целиком = 40-символьный git SHA.
var fullCommitLineRe = regexp.MustCompile(`^[0-9a-f]{40}$`)

// shortCommitLineRe — строка целиком = short SHA (7–39 hex).
var shortCommitLineRe = regexp.MustCompile(`^[0-9a-f]{7,39}$`)

// gitTagRe — явные git-теги release-* (не версия продукта M.m.p.BUILD).
var gitTagRe = regexp.MustCompile(`^(?:v?\d+\.\d+\.\d+(?:[-+][\w.-]+)?|release[-_][\w.-]+)$`)

// FromPath читает ELF и возвращает git commit из .comment, если есть.
func FromPath(path string) (string, error) {
	f, err := elf.Open(path)
	if err != nil {
		return "", fmt.Errorf("open elf: %w", err)
	}
	defer f.Close()
	return FromELF(f)
}

// FromELF извлекает git commit из открытого ELF (опционально).
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

// ParseBytes ищет git commit в .comment.
// Приоритет: tag:/commit:/git: → полный SHA (40 hex) → short SHA → release-тег.
// Версия продукта (16.0.0.1) и JIRA игнорируются, если есть SHA.
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
		if isToolchainLine(line) || isNoiseLine(line) {
			continue
		}
		if fullCommitLineRe.MatchString(line) {
			return line, nil
		}
	}

	for _, line := range lines {
		if isToolchainLine(line) || isNoiseLine(line) {
			continue
		}
		if shortCommitLineRe.MatchString(line) {
			return line, nil
		}
	}

	for _, line := range lines {
		if isToolchainLine(line) || isNoiseLine(line) {
			continue
		}
		if gitTagRe.MatchString(line) {
			return line, nil
		}
	}
	return "", ErrNotFound
}

// CommentLines возвращает непустые строки .comment (для inspect).
func CommentLines(data []byte) []string {
	return splitCommentLines(data)
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

func isNoiseLine(line string) bool {
	lower := strings.ToLower(line)
	if strings.HasPrefix(lower, "(c)") {
		return true
	}
	for _, noise := range []string{"library", "quik server", "server"} {
		if strings.Contains(lower, noise) && !strings.Contains(line, "/") {
			return true
		}
	}
	// Версия продукта M.m.p.BUILD без префикса — не git commit.
	if regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+$`).MatchString(line) {
		return true
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

// FromPathOrEmpty возвращает git commit или пустую строку.
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
