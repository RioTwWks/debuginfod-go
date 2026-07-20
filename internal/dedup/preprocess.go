package dedup

import (
	"fmt"
	"os"
	"os/exec"
)

// ToolPaths — пути к внешним утилитам (пусто = PATH).
type ToolPaths struct {
	Dwz     string
	Objcopy string
}

// Preprocessor — предобработка ELF до создания дельт.
type Preprocessor interface {
	Name() string
	Available() bool
	ApplyInPlace(path string) error
}

// NoPreprocessor — без изменений.
type NoPreprocessor struct{}

func (NoPreprocessor) Name() string    { return "none" }
func (NoPreprocessor) Available() bool { return true }
func (NoPreprocessor) ApplyInPlace(path string) error {
	return nil
}

// DecompressDwzPreprocessor — objcopy --decompress-debug-sections, затем dwz.
type DecompressDwzPreprocessor struct {
	Dwz     string
	Objcopy string
}

// NewDecompressDwzPreprocessor создаёт препроцессор decompress-dwz.
func NewDecompressDwzPreprocessor(paths ToolPaths) *DecompressDwzPreprocessor {
	dwz := paths.Dwz
	if dwz == "" {
		dwz = "dwz"
	}
	objcopy := paths.Objcopy
	if objcopy == "" {
		objcopy = "objcopy"
	}
	return &DecompressDwzPreprocessor{Dwz: dwz, Objcopy: objcopy}
}

func (d *DecompressDwzPreprocessor) Name() string { return "decompress-dwz" }

func (d *DecompressDwzPreprocessor) Available() bool {
	_, err1 := exec.LookPath(d.Dwz)
	_, err2 := exec.LookPath(d.Objcopy)
	return err1 == nil && err2 == nil
}

func (d *DecompressDwzPreprocessor) ApplyInPlace(path string) error {
	decompress := exec.Command(d.Objcopy, "--decompress-debug-sections", path)
	out, err := decompress.CombinedOutput()
	if err != nil {
		return fmt.Errorf("objcopy decompress %s: %w: %s", path, err, trimOutput(out))
	}
	cmd := exec.Command(d.Dwz, path)
	out, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("dwz %s: %w: %s", path, err, trimOutput(out))
	}
	return nil
}

// ObjcopyZstd — сжатие debug-секций zstd in-place (после дельт, только base).
type ObjcopyZstd struct {
	Bin string
}

// NewObjcopyZstd создаёт post-compress helper.
func NewObjcopyZstd(objcopy string) *ObjcopyZstd {
	if objcopy == "" {
		objcopy = "objcopy"
	}
	return &ObjcopyZstd{Bin: objcopy}
}

func (o *ObjcopyZstd) Available() bool {
	_, err := exec.LookPath(o.Bin)
	return err == nil
}

// CompressInPlace сжимает debug-секции zstd и возвращает новый размер файла.
func (o *ObjcopyZstd) CompressInPlace(path string) (int64, error) {
	cmd := exec.Command(o.Bin, "--compress-debug-sections=zstd", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("objcopy compress: %w: %s", err, trimOutput(out))
	}
	st, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return st.Size(), nil
}

// DecompressDebugSections распаковывает DWARF-секции во временную копию для xdelta decode.
func DecompressDebugSections(objcopy, srcPath, dstPath string) error {
	if objcopy == "" {
		objcopy = "objcopy"
	}
	if err := copyFileAtomic(srcPath, dstPath); err != nil {
		return err
	}
	cmd := exec.Command(objcopy, "--decompress-debug-sections", dstPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		os.Remove(dstPath)
		return fmt.Errorf("objcopy decompress %s: %w: %s", dstPath, err, trimOutput(out))
	}
	return nil
}

// DwzPreprocessor — dwz in-place (без предварительной распаковки секций).
type DwzPreprocessor struct {
	Bin string
}

// NewDwzPreprocessor создаёт dwz-препроцессор (для bench/fallback).
func NewDwzPreprocessor(dwz string) *DwzPreprocessor {
	if dwz == "" {
		dwz = "dwz"
	}
	return &DwzPreprocessor{Bin: dwz}
}

func (d *DwzPreprocessor) Name() string { return "dwz" }

func (d *DwzPreprocessor) Available() bool {
	_, err := exec.LookPath(d.Bin)
	return err == nil
}

func (d *DwzPreprocessor) ApplyInPlace(path string) error {
	cmd := exec.Command(d.Bin, path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("dwz %s: %w: %s", path, err, trimOutput(out))
	}
	return nil
}

// ResolvePreprocessor по имени стратегии.
func ResolvePreprocessor(strategy string, paths ToolPaths) Preprocessor {
	switch strategy {
	case "xdelta", "none":
		return NoPreprocessor{}
	case "dwz":
		return NewDwzPreprocessor(paths.Dwz)
	case "", "xdelta-decompress-dwz", "decompress-dwz":
		return NewDecompressDwzPreprocessor(paths)
	default:
		return NewDecompressDwzPreprocessor(paths)
	}
}
