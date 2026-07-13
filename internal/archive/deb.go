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

// ar-архив Debian: глобальный заголовок + записи по 60 байт.
func listDebELFMembers(archivePath string) ([]Member, error) {
	data, err := os.ReadFile(archivePath)
	if err != nil {
		return nil, err
	}
	if len(data) < 8 || string(data[:8]) != "!<arch>\n" {
		return nil, fmt.Errorf("invalid deb ar archive: %s", archivePath)
	}

	var members []Member
	offset := 8
	for offset+60 <= len(data) {
		header := data[offset : offset+60]
		name := strings.TrimSpace(string(header[:16]))
		sizeStr := strings.TrimSpace(string(header[48:58]))
		offset += 60

		var size int
		if _, err := fmt.Sscanf(sizeStr, "%d", &size); err != nil {
			return nil, fmt.Errorf("parse ar size for %s: %w", name, err)
		}
		if offset+size > len(data) {
			break
		}
		payload := data[offset : offset+size]
		offset += size
		if size%2 == 1 {
			offset++
		}

		if !strings.HasPrefix(name, "data.tar") {
			continue
		}

		elfs, err := listTarELFMembers(archivePath, payload, name)
		if err != nil {
			return nil, err
		}
		members = append(members, elfs...)
	}
	return members, nil
}

func listTarELFMembers(archivePath string, payload []byte, tarName string) ([]Member, error) {
	rc, err := openCompressedTar(bytes.NewReader(payload), tarName)
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
		if len(data) < 4 || data[0] != 0x7f || data[1] != 'E' || data[2] != 'L' || data[3] != 'F' {
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

type readCloser struct {
	io.Reader
	io.Closer
}
