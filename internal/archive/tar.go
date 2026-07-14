package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

func isELF(data []byte) bool {
	return len(data) >= 4 && data[0] == 0x7f && data[1] == 'E' && data[2] == 'L' && data[3] == 'F'
}

func openCompressedTar(r io.Reader, tarName string) (io.ReadCloser, error) {
	switch {
	case strings.HasSuffix(tarName, ".gz"):
		gz, err := gzip.NewReader(r)
		if err != nil {
			return nil, err
		}
		return gz, nil
	case strings.HasSuffix(tarName, ".xz"):
		xr, err := xz.NewReader(r)
		if err != nil {
			return nil, err
		}
		return io.NopCloser(xr), nil
	case strings.HasSuffix(tarName, ".zst"):
		zr, err := zstd.NewReader(r)
		if err != nil {
			return nil, err
		}
		return &zstdReadCloser{zr}, nil
	default:
		return io.NopCloser(r), nil
	}
}

type zstdReadCloser struct {
	*zstd.Decoder
}

func (z *zstdReadCloser) Close() error {
	z.Decoder.Close()
	return nil
}

func listTarELFMembers(archivePath string, payload []byte, tarName string) ([]Member, error) {
	return listTarELFMembersFromReader(archivePath, bytes.NewReader(payload), tarName)
}

func listTarELFMembersFromFile(archivePath string) ([]Member, error) {
	suffix := tarSuffix(archivePath)
	if suffix == "" {
		return nil, fmt.Errorf("unsupported tar archive: %s", archivePath)
	}
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return listTarELFMembersFromReader(archivePath, f, suffix)
}

func listTarELFMembersFromReader(archivePath string, r io.Reader, tarName string) ([]Member, error) {
	rc, err := openCompressedTar(r, tarName)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var members []Member
	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		if !isELF(data) {
			continue
		}

		memberPath := hdr.Name
		copyData := append([]byte(nil), data...)
		members = append(members, Member{
			ArchivePath: archivePath,
			MemberPath:  memberPath,
			Reader: func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(copyData)), nil
			},
		})
	}
	return members, nil
}

func listTarSourceMembersFromReader(archivePath string, r io.Reader, tarName string) ([]Member, error) {
	rc, err := openCompressedTar(r, tarName)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var members []Member
	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		if isELF(data) || !isSourceFile(hdr.Name) {
			continue
		}

		memberPath := hdr.Name
		copyData := append([]byte(nil), data...)
		members = append(members, Member{
			ArchivePath: archivePath,
			MemberPath:  memberPath,
			Reader: func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(copyData)), nil
			},
		})
	}
	return members, nil
}

func isSourceFile(name string) bool {
	lower := strings.ToLower(name)
	for _, ext := range []string{
		".c", ".h", ".cc", ".cpp", ".cxx", ".hpp", ".hh", ".hxx",
		".s", ".asm", ".rs", ".go", ".f", ".f90", ".for",
		".java", ".m", ".mm", ".swift", ".kt",
	} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}
