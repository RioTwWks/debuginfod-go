package archive

import (
	"os"
)

// APK — gzip-сжатый tar (формат Alpine Linux).
func listAPKELFMembers(archivePath string) ([]Member, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return listTarELFMembersFromReader(archivePath, f, ".tar.gz")
}
