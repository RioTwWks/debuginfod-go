package elfcomment

import (
	"debug/elf"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ErrNotFound — метка сборки не найдена в .comment.
var ErrNotFound = errors.New("build label not found in .comment")

// jiraTagRe — опциональные JIRA/DevOps метки вида DEVOPS-110 (если есть).
var jiraTagRe = regexp.MustCompile(`[A-Z][A-Z0-9_]*-\d+`)

// gitTagRe — опциональные git-теги вида v1.2.3 или release-2026-04-17.
var gitTagRe = regexp.MustCompile(`(?:^|[\s:(])(v?\d+\.\d+(?:\.\d+)?(?:[-+][\w.-]+)?|release[-_][\w.-]+)`)

// FromPath читает ELF и возвращает метку сборки из .comment, если есть.
func FromPath(path string) (string, error) {
	f, err := elf.Open(path)
	if err != nil {
		return "", fmt.Errorf("open elf: %w", err)
	}
	defer f.Close()
	return FromELF(f)
}

// FromELF извлекает метку сборки из открытого ELF (опционально).
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

// ParseBytes ищет метку сборки в .comment. Пустой результат — норма.
func ParseBytes(data []byte) (string, error) {
	text := string(data)
	if tag := jiraTagRe.FindString(text); tag != "" {
		return tag, nil
	}
	if m := gitTagRe.FindStringSubmatch(text); len(m) > 1 {
		return strings.TrimSpace(m[1]), nil
	}
	for _, line := range strings.Split(text, "\x00") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
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
	return "", ErrNotFound
}

// FromPathOrEmpty возвращает тег или пустую строку (для файлов без тега).
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
