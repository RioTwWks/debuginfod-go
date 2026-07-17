package dedup

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/klauspost/compress/zstd"
)

const zstdExt = ".zst"

// BlobStore — content-addressable хранилище сжатых blob (SHA256 → .zst).
type BlobStore struct {
	Dir string
}

// NewBlobStore создаёт CAS-хранилище в указанном каталоге.
func NewBlobStore(dir string) *BlobStore {
	return &BlobStore{Dir: dir}
}

// PathForSHA возвращает путь blob: <dir>/<sha[0:2]>/<sha>.zst.
func (b *BlobStore) PathForSHA(sha string) string {
	if len(sha) < 2 {
		return filepath.Join(b.Dir, sha+zstdExt)
	}
	return filepath.Join(b.Dir, sha[:2], sha+zstdExt)
}

// FileSHA256 вычисляет SHA256 содержимого файла.
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

// CompressFileTo сжимает src в dst (zstd) и возвращает размер сжатого файла.
func CompressFileTo(src, dst string) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return 0, err
	}
	in, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer in.Close()

	tmp := dst + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return 0, err
	}

	zw, err := zstd.NewWriter(out, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		out.Close()
		os.Remove(tmp)
		return 0, err
	}
	if _, err := io.Copy(zw, in); err != nil {
		zw.Close()
		out.Close()
		os.Remove(tmp)
		return 0, err
	}
	if err := zw.Close(); err != nil {
		out.Close()
		os.Remove(tmp)
		return 0, err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return 0, err
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return 0, err
	}
	st, err := os.Stat(dst)
	if err != nil {
		return 0, err
	}
	return st.Size(), nil
}

// DecompressFileTo распаковывает zstd-blob в outPath.
func DecompressFileTo(blobPath, outPath string) error {
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	in, err := os.Open(blobPath)
	if err != nil {
		return err
	}
	defer in.Close()

	zr, err := zstd.NewReader(in)
	if err != nil {
		return err
	}
	defer zr.Close()

	tmp := outPath + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, zr); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, outPath)
}

// VerifyDecompress проверяет, что blob распаковывается в ожидаемый SHA256.
func VerifyDecompress(blobPath, expectedSHA string) error {
	tmp, err := os.CreateTemp(filepath.Dir(blobPath), "dedup-verify-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	if err := DecompressFileTo(blobPath, tmpPath); err != nil {
		return fmt.Errorf("decompress verify: %w", err)
	}
	got, err := FileSHA256(tmpPath)
	if err != nil {
		return err
	}
	if got != expectedSHA {
		return fmt.Errorf("sha256 mismatch after decompress: got %s want %s", got, expectedSHA)
	}
	return nil
}
