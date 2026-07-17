package debugfilename

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// ErrInvalidFormat — имя файла не соответствует шаблону Quik .debug.
var ErrInvalidFormat = errors.New("invalid quik debug filename")

// Info — разобранное имя lib.so.M.m.p.BUILD.debug.
type Info struct {
	Filename  string
	Stem      string
	Version   string
	BuildNum  int
}

// Parse разбирает имя файла вида lib.so.19.1.5.2899.debug.
func Parse(name string) (Info, error) {
	base := filepath.Base(name)
	if !strings.HasSuffix(strings.ToLower(base), ".debug") {
		return Info{}, fmt.Errorf("%w: missing .debug suffix", ErrInvalidFormat)
	}
	without := strings.TrimSuffix(base, filepath.Ext(base))
	parts := strings.Split(without, ".")
	if len(parts) < 5 {
		return Info{}, fmt.Errorf("%w: need stem.M.m.p.BUILD", ErrInvalidFormat)
	}

	buildNum, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil || buildNum < 0 {
		return Info{}, fmt.Errorf("%w: invalid build number", ErrInvalidFormat)
	}
	for i := len(parts) - 4; i < len(parts)-1; i++ {
		if _, err := strconv.Atoi(parts[i]); err != nil {
			return Info{}, fmt.Errorf("%w: invalid version segment", ErrInvalidFormat)
		}
	}
	version := strings.Join(parts[len(parts)-4:len(parts)-1], ".")
	stem := strings.Join(parts[:len(parts)-4], ".")
	if stem == "" || version == "" {
		return Info{}, fmt.Errorf("%w: empty stem or version", ErrInvalidFormat)
	}
	return Info{
		Filename: base,
		Stem:     stem,
		Version:  version,
		BuildNum: buildNum,
	}, nil
}

// ParseBuildDir извлекает номер из каталога build_482_2025-03-26_….
func ParseBuildDir(dirName string) (int, error) {
	name := filepath.Base(dirName)
	if !strings.HasPrefix(name, "build_") {
		return 0, fmt.Errorf("%w: not a build_* directory", ErrInvalidFormat)
	}
	rest := strings.TrimPrefix(name, "build_")
	idx := strings.Index(rest, "_")
	if idx <= 0 {
		return 0, fmt.Errorf("%w: missing build number", ErrInvalidFormat)
	}
	num, err := strconv.Atoi(rest[:idx])
	if err != nil || num < 0 {
		return 0, fmt.Errorf("%w: invalid build directory number", ErrInvalidFormat)
	}
	return num, nil
}
