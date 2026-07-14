package buildid

import (
	"bytes"
	"debug/elf"
	"errors"
	"fmt"
	"os"
	"strings"
)

// ErrNotFound возвращается, если в ELF-файле нет GNU/Go build-id.
var ErrNotFound = errors.New("build-id not found")

// FromBytes извлекает build-id из ELF-данных в памяти.
func FromBytes(data []byte) (Result, error) {
	f, err := elf.NewFile(bytes.NewReader(data))
	if err != nil {
		return Result{}, err
	}
	return FromELF(f)
}

// ArtifactTypeFromBytes определяет тип артефакта по пути-намёку и ELF-данным.
func ArtifactTypeFromBytes(pathHint string, data []byte) (string, error) {
	f, err := elf.NewFile(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	return ArtifactType(pathHint, f), nil
}

// FromPath извлекает build-id из ELF-файла по пути.
func FromPath(path string) (string, error) {
	result, err := FromPathDetailed(path)
	if err != nil {
		return "", err
	}
	return result.Value, nil
}

// FromPathDetailed возвращает build-id вместе с типом заметки.
func FromPathDetailed(path string) (Result, error) {
	f, err := elf.Open(path)
	if err != nil {
		return Result{}, err
	}
	defer f.Close()
	return FromELF(f)
}

// FromELF извлекает build-id из уже открытого ELF-файла.
func FromELF(f *elf.File) (Result, error) {
	for _, sec := range f.Sections {
		if sec.Type != elf.SHT_NOTE {
			continue
		}
		data, err := sec.Data()
		if err != nil {
			continue
		}
		result, err := parseNotes(data)
		if err == nil {
			return result, nil
		}
	}
	return Result{}, ErrNotFound
}

// Normalize приводит build-id к формату debuginfod: lowercase hex без префиксов.
func Normalize(id string) string {
	id = strings.TrimPrefix(strings.ToLower(id), "0x")
	id = strings.ReplaceAll(id, "-", "")
	return id
}

// IsELF проверяет, является ли файл ELF по magic bytes.
func IsELF(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	header := make([]byte, 4)
	if _, err := f.Read(header); err != nil {
		return false
	}
	return len(header) == 4 && header[0] == 0x7f && header[1] == 'E' &&
		header[2] == 'L' && header[3] == 'F'
}

// ArtifactType определяет, является ли ELF executable или debuginfo.
func ArtifactType(path string, f *elf.File) string {
	base := strings.ToLower(path)
	if strings.HasSuffix(base, ".debug") {
		return "debuginfo"
	}
	if strings.Contains(base, "/.build-id/") {
		return "debuginfo"
	}
	if strings.Contains(base, "/usr/lib/debug/") {
		return "debuginfo"
	}

	switch f.Type {
	case elf.ET_EXEC, elf.ET_DYN:
		return "executable"
	default:
		return "debuginfo"
	}
}

// OpenELF открывает ELF-файл и возвращает понятную ошибку.
func OpenELF(path string) (*elf.File, error) {
	f, err := elf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open elf %s: %w", path, err)
	}
	return f, nil
}
