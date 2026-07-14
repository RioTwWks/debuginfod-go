package archive

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
)

func openTarArchiveMember(archivePath, memberPath, suffix string) (io.ReadCloser, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rc, err := openCompressedTar(f, suffix)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("member %q not found in %s", memberPath, archivePath)
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
