package elfsection

import (
	"debug/elf"
	"errors"
	"fmt"
)

var (
	// ErrNotFound — секция не найдена в ELF.
	ErrNotFound = errors.New("section not found")
	// ErrNoBits — секция SHT_NOBITS без данных в файле.
	ErrNoBits = errors.New("section has no bits")
)

// Extract читает сырые данные секции из ELF-файла по имени.
func Extract(elfPath, sectionName string) ([]byte, error) {
	f, err := elf.Open(elfPath)
	if err != nil {
		return nil, fmt.Errorf("open elf: %w", err)
	}
	defer f.Close()
	return FromELF(f, sectionName)
}

// FromELF извлекает секцию из открытого ELF.
func FromELF(f *elf.File, sectionName string) ([]byte, error) {
	sec := f.Section(sectionName)
	if sec == nil {
		return nil, ErrNotFound
	}
	if sec.Type == elf.SHT_NOBITS {
		return nil, ErrNoBits
	}
	data, err := sec.Data()
	if err != nil {
		return nil, fmt.Errorf("read section %s: %w", sectionName, err)
	}
	return data, nil
}

// ExtractFirst ищет секцию сначала в debuginfo, затем в executable.
func ExtractFirst(debuginfoPath, executablePath, sectionName string) ([]byte, error) {
	if debuginfoPath != "" {
		data, err := Extract(debuginfoPath, sectionName)
		if err == nil {
			return data, nil
		}
		if !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrNoBits) {
			return nil, err
		}
	}
	if executablePath != "" {
		return Extract(executablePath, sectionName)
	}
	return nil, ErrNotFound
}
