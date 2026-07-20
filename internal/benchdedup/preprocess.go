package benchdedup

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/your-username/debuginfod-go/internal/dedup"
)

// Preprocessor — предобработка ELF до создания дельт (копия в workDir).
type Preprocessor interface {
	Name() string
	Available() bool
	Prepare(workDir, src string) (string, error)
}

type copyPreprocessor struct {
	inner dedup.Preprocessor
}

func (c copyPreprocessor) Name() string    { return c.inner.Name() }
func (c copyPreprocessor) Available() bool { return c.inner.Available() }

func (c copyPreprocessor) Prepare(workDir, src string) (string, error) {
	dst := filepath.Join(workDir, filepath.Base(src))
	if err := copyFile(src, dst); err != nil {
		return "", err
	}
	if err := c.inner.ApplyInPlace(dst); err != nil {
		return "", err
	}
	return dst, nil
}

// ObjcopyZstdPost — сжатие debug-секций zstd в копии (после дельт).
type ObjcopyZstdPost struct {
	inner *dedup.ObjcopyZstd
}

// NewObjcopyZstdPost создаёт post-compress helper для bench.
func NewObjcopyZstdPost(bin string) *ObjcopyZstdPost {
	return &ObjcopyZstdPost{inner: dedup.NewObjcopyZstd(bin)}
}

func (o *ObjcopyZstdPost) Available() bool {
	return o.inner.Available()
}

// CompressInPlace копирует src в workDir и сжимает debug-секции zstd.
func (o *ObjcopyZstdPost) CompressInPlace(workDir, src string) (string, int64, error) {
	dst := filepath.Join(workDir, "compressed_"+filepath.Base(src))
	if err := copyFile(src, dst); err != nil {
		return "", 0, err
	}
	size, err := o.inner.CompressInPlace(dst)
	if err != nil {
		return "", 0, err
	}
	return dst, size, nil
}

// ResolvePreprocessors по именам (делегирует в internal/dedup).
func ResolvePreprocessors(names []string, paths ToolPaths) []Preprocessor {
	tools := dedup.ToolPaths{Dwz: paths.Dwz, Objcopy: paths.Objcopy}
	var out []Preprocessor
	for _, name := range names {
		switch name {
		case "none", "":
			out = append(out, copyPreprocessor{inner: dedup.NoPreprocessor{}})
		case "dwz":
			out = append(out, copyPreprocessor{inner: dedup.NewDwzPreprocessor(paths.Dwz)})
		case "decompress-dwz", "dwz-decompress":
			out = append(out, copyPreprocessor{inner: dedup.NewDecompressDwzPreprocessor(tools)})
		}
	}
	if len(out) == 0 {
		out = append(out, copyPreprocessor{inner: dedup.NoPreprocessor{}})
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
