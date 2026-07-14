package archive

import (
	"os"
)

func listPacmanELFMembers(archivePath string) ([]Member, error) {
	suffix := tarSuffix(archivePath)
	if suffix == "" {
		suffix = ".tar.zst"
	}
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return listTarELFMembersFromReader(archivePath, f, suffix)
}
