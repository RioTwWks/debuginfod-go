package benchdedup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Preprocessor — предобработка ELF до создания дельт.
type Preprocessor interface {
	Name() string
	Available() bool
	// Prepare копирует src в workDir и применяет препроцессор; возвращает путь к файлу.
	Prepare(workDir, src string) (string, error)
}

// NoPreprocessor — без изменений (копия в workDir).
type NoPreprocessor struct{}

func (NoPreprocessor) Name() string      { return "none" }
func (NoPreprocessor) Available() bool   { return true }
func (n NoPreprocessor) Prepare(workDir, src string) (string, error) {
	dst := filepath.Join(workDir, filepath.Base(src))
	if err := copyFile(src, dst); err != nil {
		return "", err
	}
	return dst, nil
}

// DwzPreprocessor — dwz на копии файла (in-place).
type DwzPreprocessor struct {
	Bin string
}

func NewDwzPreprocessor(bin string) *DwzPreprocessor {
	if bin == "" {
		bin = "dwz"
	}
	return &DwzPreprocessor{Bin: bin}
}

func (d *DwzPreprocessor) Name() string { return "dwz" }

func (d *DwzPreprocessor) Available() bool {
	_, err := exec.LookPath(d.Bin)
	return err == nil
}

func (d *DwzPreprocessor) Prepare(workDir, src string) (string, error) {
	dst := filepath.Join(workDir, filepath.Base(src))
	if err := copyFile(src, dst); err != nil {
		return "", err
	}
	cmd := exec.Command(d.Bin, dst)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("dwz %s: %w: %s", dst, err, trimOutput(out))
	}
	return dst, nil
}

// ObjcopyZstdPost — сжатие debug-секций ПОСЛЕ дельт (отдельный эксперимент).
type ObjcopyZstdPost struct {
	Bin string
}

func NewObjcopyZstdPost(bin string) *ObjcopyZstdPost {
	if bin == "" {
		bin = "objcopy"
	}
	return &ObjcopyZstdPost{Bin: bin}
}

func (o *ObjcopyZstdPost) Name() string { return "objcopy-zstd" }

func (o *ObjcopyZstdPost) Available() bool {
	_, err := exec.LookPath(o.Bin)
	return err == nil
}

// CompressInPlace сжимает debug-секции zstd в копии файла.
func (o *ObjcopyZstdPost) CompressInPlace(workDir, src string) (string, int64, error) {
	dst := filepath.Join(workDir, "compressed_"+filepath.Base(src))
	if err := copyFile(src, dst); err != nil {
		return "", 0, err
	}
	cmd := exec.Command(o.Bin, "--compress-debug-sections=zstd", dst)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", 0, fmt.Errorf("objcopy compress: %w: %s", err, trimOutput(out))
	}
	size, err := fileSize(dst)
	if err != nil {
		return "", 0, err
	}
	return dst, size, nil
}

// ResolvePreprocessors по именам.
func ResolvePreprocessors(names []string, paths ToolPaths) []Preprocessor {
	var out []Preprocessor
	for _, name := range names {
		switch name {
		case "none", "":
			out = append(out, NoPreprocessor{})
		case "dwz":
			out = append(out, NewDwzPreprocessor(paths.Dwz))
		}
	}
	if len(out) == 0 {
		out = append(out, NoPreprocessor{})
	}
	return out
}

// PrepareGroupFiles копирует/препроцессит все файлы группы в workDir.
func PrepareGroupFiles(pp Preprocessor, workDir string, files []DebugFile) ([]string, error) {
	paths := make([]string, len(files))
	for i, f := range files {
		sub := filepath.Join(workDir, fmt.Sprintf("f%d", i))
		if err := os.MkdirAll(sub, 0o755); err != nil {
			return nil, err
		}
		p, err := pp.Prepare(sub, f.Path)
		if err != nil {
			return nil, fmt.Errorf("preprocess %s: %w", f.Path, err)
		}
		paths[i] = p
	}
	return paths, nil
}
