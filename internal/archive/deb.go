package archive

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
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

func openDebMember(archivePath, memberPath string) (io.ReadCloser, error) {
	data, err := os.ReadFile(archivePath)
	if err != nil {
		return nil, err
	}
	if len(data) < 8 || string(data[:8]) != "!<arch>\n" {
		return nil, fmt.Errorf("invalid deb ar archive: %s", archivePath)
	}

	offset := 8
	for offset+60 <= len(data) {
		header := data[offset : offset+60]
		name := strings.TrimSpace(string(header[:16]))
		sizeStr := strings.TrimSpace(string(header[48:58]))
		offset += 60

		var size int
		if _, err := fmt.Sscanf(sizeStr, "%d", &size); err != nil {
			return nil, err
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

		rc, err := openCompressedTar(bytes.NewReader(payload), name)
		if err != nil {
			return nil, err
		}
		defer rc.Close()

		tr := tar.NewReader(rc)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			if hdr.Name == memberPath || strings.TrimPrefix(hdr.Name, "./") == memberPath {
				data, err := io.ReadAll(tr)
				if err != nil {
					return nil, err
				}
				return io.NopCloser(bytes.NewReader(data)), nil
			}
		}
	}
	return nil, fmt.Errorf("member %q not found in %s", memberPath, archivePath)
}
