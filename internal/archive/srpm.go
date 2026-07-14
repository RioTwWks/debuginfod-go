package archive

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sassoftware/go-rpmutils"
)

func listSRPMSourceMembers(archivePath string) ([]Member, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rpm, err := rpmutils.ReadRpm(f)
	if err != nil {
		return nil, fmt.Errorf("read srpm %s: %w", archivePath, err)
	}

	reader, err := rpm.PayloadReaderExtended()
	if err != nil {
		return nil, err
	}

	var members []Member
	for {
		info, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if reader.IsLink() {
			continue
		}

		name := strings.TrimPrefix(info.Name(), "./")
		data, err := io.ReadAll(reader)
		if err != nil {
			continue
		}

		if isELF(data) {
			continue
		}

		lower := strings.ToLower(name)
		switch {
		case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
			nested, err := listTarSourceMembersFromReader(archivePath, bytes.NewReader(data), ".tar.gz")
			if err != nil {
				continue
			}
			for i := range nested {
				nested[i].MemberPath = name + "!" + nested[i].MemberPath
			}
			members = append(members, nested...)
		case strings.HasSuffix(lower, ".tar.xz"):
			nested, err := listTarSourceMembersFromReader(archivePath, bytes.NewReader(data), ".tar.xz")
			if err != nil {
				continue
			}
			for i := range nested {
				nested[i].MemberPath = name + "!" + nested[i].MemberPath
			}
			members = append(members, nested...)
		case strings.HasSuffix(lower, ".tar.zst"):
			nested, err := listTarSourceMembersFromReader(archivePath, bytes.NewReader(data), ".tar.zst")
			if err != nil {
				continue
			}
			for i := range nested {
				nested[i].MemberPath = name + "!" + nested[i].MemberPath
			}
			members = append(members, nested...)
		case strings.HasSuffix(lower, ".tar"):
			nested, err := listTarSourceMembersFromReader(archivePath, bytes.NewReader(data), ".tar")
			if err != nil {
				continue
			}
			for i := range nested {
				nested[i].MemberPath = name + "!" + nested[i].MemberPath
			}
			members = append(members, nested...)
		case isSourceFile(name):
			copyData := append([]byte(nil), data...)
			members = append(members, Member{
				ArchivePath: archivePath,
				MemberPath:  name,
				Reader: func() (io.ReadCloser, error) {
					return io.NopCloser(bytes.NewReader(copyData)), nil
				},
			})
		}
	}
	return members, nil
}

// openNestedMember открывает член SRPM с вложенным tar (memberPath вида "foo.tar.gz!path/to/file.c").
func openNestedMember(archivePath, memberPath string) (io.ReadCloser, error) {
	parts := strings.SplitN(memberPath, "!", 2)
	if len(parts) != 2 {
		return openSRPMMember(archivePath, memberPath)
	}

	outerData, err := readSRPMMember(archivePath, parts[0])
	if err != nil {
		return nil, err
	}

	suffix := tarSuffix(parts[0])
	if suffix == "" {
		suffix = ".tar"
	}
	rc, err := openCompressedTar(bytes.NewReader(outerData), suffix)
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
		if hdr.Name == parts[1] || strings.TrimPrefix(hdr.Name, "./") == parts[1] {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			return io.NopCloser(bytes.NewReader(data)), nil
		}
	}
}

func readSRPMMember(archivePath, memberPath string) ([]byte, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rpm, err := rpmutils.ReadRpm(f)
	if err != nil {
		return nil, err
	}
	reader, err := rpm.PayloadReaderExtended()
	if err != nil {
		return nil, err
	}

	for {
		info, err := reader.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("member %q not found", memberPath)
		}
		if err != nil {
			return nil, err
		}
		name := strings.TrimPrefix(info.Name(), "./")
		if name != memberPath {
			continue
		}
		return io.ReadAll(reader)
	}
}

func openSRPMMember(archivePath, memberPath string) (io.ReadCloser, error) {
	data, err := readSRPMMember(archivePath, memberPath)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}
