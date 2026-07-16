package dedup

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// Xdelta — обёртка над xdelta3 CLI.
type Xdelta struct {
	Bin string
}

// NewXdelta создаёт runner с путём к бинарнику.
func NewXdelta(bin string) *Xdelta {
	if bin == "" {
		bin = "xdelta3"
	}
	return &Xdelta{Bin: bin}
}

// Available проверяет наличие xdelta3.
func (x *Xdelta) Available() bool {
	_, err := exec.LookPath(x.Bin)
	return err == nil
}

// Encode создаёт delta: base + target → deltaPath.
func (x *Xdelta) Encode(basePath, targetPath, deltaPath string) error {
	if err := os.MkdirAll(filepath.Dir(deltaPath), 0o755); err != nil {
		return err
	}
	cmd := exec.Command(x.Bin, "-e", "-s", basePath, targetPath, deltaPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("xdelta3 encode: %w: %s", err, trimOutput(out))
	}
	return nil
}

// Decode восстанавливает target из base + delta.
func (x *Xdelta) Decode(basePath, deltaPath, outPath string) error {
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	cmd := exec.Command(x.Bin, "-d", "-s", basePath, deltaPath, outPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("xdelta3 decode: %w: %s", err, trimOutput(out))
	}
	return nil
}

// FileSHA256 вычисляет SHA256 файла.
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func trimOutput(b []byte) string {
	const max = 512
	s := string(b)
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}

// DeltaPathFor возвращает путь для .xdelta рядом с оригиналом.
func DeltaPathFor(filePath string) string {
	return filePath + ".xdelta"
}
