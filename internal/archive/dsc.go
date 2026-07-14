package archive

import (
	"archive/tar"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func listDSCSourceMembers(dscPath string) ([]Member, error) {
	files, err := parseDSCFiles(dscPath)
	if err != nil {
		return nil, err
	}

	dir := filepath.Dir(dscPath)
	var members []Member
	for _, name := range files {
		lower := strings.ToLower(name)
		fullPath := filepath.Join(dir, name)
		switch {
		case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
			f, err := os.Open(fullPath)
			if err != nil {
				continue
			}
			nested, err := listTarSourceMembersFromReader(dscPath, f, ".tar.gz")
			f.Close()
			if err != nil {
				continue
			}
			for i := range nested {
				nested[i].MemberPath = name + "!" + nested[i].MemberPath
			}
			members = append(members, nested...)
		case strings.HasSuffix(lower, ".tar.xz"):
			f, err := os.Open(fullPath)
			if err != nil {
				continue
			}
			nested, err := listTarSourceMembersFromReader(dscPath, f, ".tar.xz")
			f.Close()
			if err != nil {
				continue
			}
			for i := range nested {
				nested[i].MemberPath = name + "!" + nested[i].MemberPath
			}
			members = append(members, nested...)
		case strings.HasSuffix(lower, ".tar.zst"):
			f, err := os.Open(fullPath)
			if err != nil {
				continue
			}
			nested, err := listTarSourceMembersFromReader(dscPath, f, ".tar.zst")
			f.Close()
			if err != nil {
				continue
			}
			for i := range nested {
				nested[i].MemberPath = name + "!" + nested[i].MemberPath
			}
			members = append(members, nested...)
		case strings.HasSuffix(lower, ".tar"):
			f, err := os.Open(fullPath)
			if err != nil {
				continue
			}
			nested, err := listTarSourceMembersFromReader(dscPath, f, ".tar")
			f.Close()
			if err != nil {
				continue
			}
			for i := range nested {
				nested[i].MemberPath = name + "!" + nested[i].MemberPath
			}
			members = append(members, nested...)
		}
	}
	return members, nil
}

func parseDSCFiles(dscPath string) ([]string, error) {
	f, err := os.Open(dscPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	inFiles := false
	var files []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Files:") {
			inFiles = true
			rest := strings.TrimSpace(strings.TrimPrefix(line, "Files:"))
			if rest != "" {
				if name := dscFileName(rest); name != "" {
					files = append(files, name)
				}
			}
			continue
		}
		if inFiles {
			if line == "" || !strings.HasPrefix(line, " ") {
				break
			}
			trimmed := strings.TrimSpace(line)
			if name := dscFileName(trimmed); name != "" {
				files = append(files, name)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no Files in dsc: %s", dscPath)
	}
	return files, nil
}

// dscFileName извлекает имя файла из строки Files (формат: "<size> <hash> <name>").
func dscFileName(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}

func openDSCMember(dscPath, memberPath string) (io.ReadCloser, error) {
	parts := strings.SplitN(memberPath, "!", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid dsc member path: %s", memberPath)
	}

	tarPath := filepath.Join(filepath.Dir(dscPath), parts[0])
	f, err := os.Open(tarPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	suffix := tarSuffix(parts[0])
	if suffix == "" {
		suffix = ".tar"
	}
	rc, err := openCompressedTar(f, suffix)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("member %q not found", memberPath)
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
