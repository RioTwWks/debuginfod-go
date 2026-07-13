package buildid

import (
	"debug/elf"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
)

// ErrNotFound возвращается, если в ELF-файле нет GNU build-id.
var ErrNotFound = errors.New("build-id not found")

// FromPath извлекает build-id из ELF-файла по пути.
func FromPath(path string) (string, error) {
	f, err := elf.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	return FromELF(f)
}

// FromELF извлекает build-id из уже открытого ELF-файла.
func FromELF(f *elf.File) (string, error) {
	for _, sec := range f.Sections {
		if sec.Type != elf.SHT_NOTE {
			continue
		}
		data, err := sec.Data()
		if err != nil {
			continue
		}
		id, err := parseNoteSection(data)
		if err == nil {
			return id, nil
		}
	}
	return "", ErrNotFound
}

// Normalize приводит build-id к формату debuginfod: lowercase hex без префиксов.
func Normalize(id string) string {
	id = strings.TrimPrefix(strings.ToLower(id), "0x")
	id = strings.ReplaceAll(id, "-", "")
	return id
}

// parseNoteSection ищет NT_GNU_BUILD_ID в секции SHT_NOTE.
func parseNoteSection(data []byte) (string, error) {
	offset := 0
	for offset < len(data) {
		if offset+12 > len(data) {
			break
		}

		namesz := int(uint32(data[offset]) | uint32(data[offset+1])<<8 |
			uint32(data[offset+2])<<16 | uint32(data[offset+3])<<24)
		descsz := int(uint32(data[offset+4]) | uint32(data[offset+5])<<8 |
			uint32(data[offset+6])<<16 | uint32(data[offset+7])<<24)
		typ := int(uint32(data[offset+8]) | uint32(data[offset+9])<<8 |
			uint32(data[offset+10])<<16 | uint32(data[offset+11])<<24)
		offset += 12

		namePad := align4(namesz)
		descPad := align4(descsz)
		if offset+namePad+descsz > len(data) {
			break
		}

		name := string(data[offset : offset+namesz])
		offset += namePad

		desc := data[offset : offset+descsz]
		offset += descPad

		// NT_GNU_BUILD_ID = 3, владелец заметки — "GNU".
		if typ == 3 && strings.TrimRight(name, "\x00") == "GNU" && len(desc) > 0 {
			return hex.EncodeToString(desc), nil
		}
	}
	return "", ErrNotFound
}

func align4(n int) int {
	if n%4 == 0 {
		return n
	}
	return n + (4 - n%4)
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
