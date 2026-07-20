package benchdedup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/your-username/debuginfod-go/internal/dedup"
)

// DiffAlgo — интерфейс дифференциального сжатия (внешний CLI).
type DiffAlgo interface {
	Name() string
	Available() bool
	Encode(basePath, targetPath, patchPath string) error
	Decode(basePath, patchPath, outPath string) error
}

type xdeltaAlgo struct {
	x *dedup.Xdelta
}

func (x *xdeltaAlgo) Name() string { return "xdelta3" }

func (x *xdeltaAlgo) Available() bool { return x.x.Available() }

func (x *xdeltaAlgo) Encode(basePath, targetPath, patchPath string) error {
	return x.x.Encode(basePath, targetPath, patchPath)
}

func (x *xdeltaAlgo) Decode(basePath, patchPath, outPath string) error {
	return x.x.Decode(basePath, patchPath, outPath)
}

// Bsdiff — обёртка bsdiff/bspatch.
type Bsdiff struct {
	EncodeBin string
	DecodeBin string
}

func NewBsdiff(encodeBin, decodeBin string) *Bsdiff {
	if encodeBin == "" {
		encodeBin = "bsdiff"
	}
	if decodeBin == "" {
		decodeBin = "bspatch"
	}
	return &Bsdiff{EncodeBin: encodeBin, DecodeBin: decodeBin}
}

func (b *Bsdiff) Name() string { return "bsdiff" }

func (b *Bsdiff) Available() bool {
	_, err1 := exec.LookPath(b.EncodeBin)
	_, err2 := exec.LookPath(b.DecodeBin)
	return err1 == nil && err2 == nil
}

func (b *Bsdiff) Encode(basePath, targetPath, patchPath string) error {
	if err := ensureDir(filepath.Dir(patchPath)); err != nil {
		return err
	}
	cmd := exec.Command(b.EncodeBin, basePath, targetPath, patchPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bsdiff encode: %w: %s", err, trimOutput(out))
	}
	return nil
}

func (b *Bsdiff) Decode(basePath, patchPath, outPath string) error {
	if err := ensureDir(filepath.Dir(outPath)); err != nil {
		return err
	}
	cmd := exec.Command(b.DecodeBin, basePath, patchPath, outPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bspatch decode: %w: %s", err, trimOutput(out))
	}
	return nil
}

// HDiffPatch — обёртка hdiffz/hpatchz.
type HDiffPatch struct {
	EncodeBin string
	DecodeBin string
}

func NewHDiffPatch(encodeBin, decodeBin string) *HDiffPatch {
	if encodeBin == "" {
		encodeBin = "hdiffz"
	}
	if decodeBin == "" {
		decodeBin = "hpatchz"
	}
	return &HDiffPatch{EncodeBin: encodeBin, DecodeBin: decodeBin}
}

func (h *HDiffPatch) Name() string { return "hdiffpatch" }

func (h *HDiffPatch) Available() bool {
	_, err1 := exec.LookPath(h.EncodeBin)
	_, err2 := exec.LookPath(h.DecodeBin)
	return err1 == nil && err2 == nil
}

func (h *HDiffPatch) Encode(basePath, targetPath, patchPath string) error {
	if err := ensureDir(filepath.Dir(patchPath)); err != nil {
		return err
	}
	// -f: force overwrite; -m: match mode for large binaries
	cmd := exec.Command(h.EncodeBin, "-f", "-m", basePath, targetPath, patchPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("hdiffz encode: %w: %s", err, trimOutput(out))
	}
	return nil
}

func (h *HDiffPatch) Decode(basePath, patchPath, outPath string) error {
	if err := ensureDir(filepath.Dir(outPath)); err != nil {
		return err
	}
	cmd := exec.Command(h.DecodeBin, "-f", basePath, patchPath, outPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("hpatchz decode: %w: %s", err, trimOutput(out))
	}
	return nil
}

// ResolveAlgos возвращает список алгоритмов по именам.
func ResolveAlgos(names []string, paths ToolPaths) []DiffAlgo {
	var out []DiffAlgo
	for _, name := range names {
		switch name {
		case "xdelta3", "xdelta":
			out = append(out, &xdeltaAlgo{x: dedup.NewXdelta(paths.Xdelta3)})
		case "bsdiff":
			out = append(out, NewBsdiff(paths.Bsdiff, paths.Bspatch))
		case "hdiff", "hdiffpatch":
			out = append(out, NewHDiffPatch(paths.Hdiffz, paths.Hpatchz))
		}
	}
	return out
}

// ToolPaths — пути к внешним утилитам (пусто = PATH).
type ToolPaths struct {
	Xdelta3 string
	Bsdiff  string
	Bspatch string
	Hdiffz  string
	Hpatchz string
	Dwz     string
	Objcopy string
}

// CheckTools возвращает отчёт о доступности утилит.
func CheckTools(paths ToolPaths) map[string]bool {
	algos := []DiffAlgo{
		&xdeltaAlgo{x: dedup.NewXdelta(paths.Xdelta3)},
		NewBsdiff(paths.Bsdiff, paths.Bspatch),
		NewHDiffPatch(paths.Hdiffz, paths.Hpatchz),
	}
	out := make(map[string]bool, 8)
	for _, a := range algos {
		out[a.Name()] = a.Available()
	}
	out["dwz"] = toolAvailable(paths.Dwz, "dwz")
	out["objcopy"] = toolAvailable(paths.Objcopy, "objcopy")
	return out
}

func toolAvailable(override, fallback string) bool {
	bin := override
	if bin == "" {
		bin = fallback
	}
	_, err := exec.LookPath(bin)
	return err == nil
}

// PatchExt возвращает расширение патча для алгоритма.
func PatchExt(algo string) string {
	switch algo {
	case "bsdiff":
		return ".bsdiff"
	case "hdiffpatch":
		return ".hdiff"
	default:
		return ".xdelta"
	}
}

// TempPatchPath создаёт путь патча во временном каталоге.
func TempPatchPath(dir, algo, label string) string {
	return filepath.Join(dir, safeName(algo, label)+PatchExt(algo))
}

// RemoveIfExists удаляет файл, игнорируя отсутствие.
func RemoveIfExists(path string) {
	_ = os.Remove(path)
}
