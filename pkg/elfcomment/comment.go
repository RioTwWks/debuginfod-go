package elfcomment

import (
	"debug/elf"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ErrNotFound — commit tag не найден в .comment.
var ErrNotFound = errors.New("commit tag not found in .comment")

// commitTagRe — JIRA/DevOps теги вида DEVOPS-110, PROJ-42.
var commitTagRe = regexp.MustCompile(`[A-Z][A-Z0-9_]*-\d+`)

// FromPath читает ELF и возвращает первый commit tag из секции .comment.
func FromPath(path string) (string, error) {
	f, err := elf.Open(path)
	if err != nil {
		return "", fmt.Errorf("open elf: %w", err)
	}
	defer f.Close()
	return FromELF(f)
}

// FromELF извлекает commit tag из открытого ELF.
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

// ParseBytes ищет commit tag в сырых данных .comment.
func ParseBytes(data []byte) (string, error) {
	text := string(data)
	if tag := commitTagRe.FindString(text); tag != "" {
		return tag, nil
	}
	// Fallback: первая непустая строка после "tag:" или "commit:"
	for _, line := range strings.Split(text, "\x00") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		for _, prefix := range []string{"tag:", "commit:", "build:"} {
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
