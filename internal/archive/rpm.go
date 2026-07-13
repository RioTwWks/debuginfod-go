package archive

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sassoftware/go-rpmutils"
)

func listRPMELFMembers(archivePath string) ([]Member, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rpm, err := rpmutils.ReadRpm(f)
	if err != nil {
		return nil, fmt.Errorf("read rpm %s: %w", archivePath, err)
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
		if len(data) < 4 || data[0] != 0x7f || data[1] != 'E' || data[2] != 'L' || data[3] != 'F' {
			continue
		}

		copyData := append([]byte(nil), data...)
		members = append(members, Member{
			ArchivePath: archivePath,
			MemberPath:  name,
			Reader: func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(copyData)), nil
			},
		})
	}
	return members, nil
}
