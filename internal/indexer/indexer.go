package indexer

import (
	"debug/dwarf"
	"debug/elf"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/your-username/debuginfod-go/internal/archive"
	"github.com/your-username/debuginfod-go/internal/storage"
	"github.com/your-username/debuginfod-go/pkg/buildid"
)

// Indexer сканирует файловую систему и заполняет SQLite-индекс.
type Indexer struct {
	storage  *storage.Storage
	paths    []string
	cacheDir string
}

// NewIndexer создаёт индексатор для указанных корневых путей.
func NewIndexer(store *storage.Storage, paths []string, cacheDir string) *Indexer {
	return &Indexer{storage: store, paths: paths, cacheDir: cacheDir}
}

// Scan обходит все пути и индексирует ELF-файлы и архивы.
// Файлы с неизменившимися mtime/size пропускаются (инкрементальная индексация).
func (i *Indexer) Scan() error {
	var indexed, skipped int
	for _, root := range i.paths {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				log.Printf("пропуск %s: %v", path, err)
				return nil
			}
			if entry.IsDir() {
				return nil
			}

			info, err := entry.Info()
			if err != nil {
				return nil
			}
			mtime := info.ModTime().UnixNano()
			size := info.Size()

			if archive.IsArchive(path) {
				needs, err := i.storage.NeedsScan(path, mtime, size)
				if err != nil {
					log.Printf("NeedsScan %s: %v", path, err)
				} else if !needs {
					skipped++
					return nil
				}
				count, err := i.indexArchive(path, mtime)
				if err != nil {
					log.Printf("ошибка индексации архива %s: %v", path, err)
					return nil
				}
				if err := i.storage.MarkScanned(path, mtime, size, "archive"); err != nil {
					log.Printf("MarkScanned %s: %v", path, err)
				}
				indexed += count
				return nil
			}

			if buildid.IsELF(path) {
				needs, err := i.storage.NeedsScan(path, mtime, size)
				if err != nil {
					log.Printf("NeedsScan %s: %v", path, err)
				} else if !needs {
					skipped++
					return nil
				}
				if err := i.indexELF(path, mtime, "", ""); err != nil {
					log.Printf("ошибка индексации ELF %s: %v", path, err)
					return nil
				}
				if err := i.storage.MarkScanned(path, mtime, size, "elf"); err != nil {
					log.Printf("MarkScanned %s: %v", path, err)
				}
				indexed++
			}
			return nil
		})
		if err != nil {
			log.Printf("ошибка обхода %s: %v", root, err)
		}
	}
	log.Printf("индексация завершена: новых/обновлённых ELF %d, пропущено без изменений %d", indexed, skipped)
	return nil
}

func (i *Indexer) indexArchive(archivePath string, mtimeNS int64) (int, error) {
	members, err := archive.ListELFMembers(archivePath)
	if err != nil {
		return 0, err
	}

	var count int
	for _, member := range members {
		cached, err := archive.ExtractToCache(i.cacheDir, member.ArchivePath, member.MemberPath, member.Reader)
		if err != nil {
			log.Printf("не удалось извлечь %s:%s: %v", member.ArchivePath, member.MemberPath, err)
			continue
		}
		if err := i.indexELF(cached, mtimeNS, member.ArchivePath, member.MemberPath); err != nil {
			log.Printf("ошибка индексации %s в %s: %v", member.MemberPath, archivePath, err)
			continue
		}
		count++
	}
	return count, nil
}

func (i *Indexer) indexELF(path string, mtimeNS int64, archivePath, memberPath string) error {
	result, err := buildid.FromPathDetailed(path)
	if err != nil {
		return err
	}
	result.Value = buildid.Normalize(result.Value)

	elfFile, err := buildid.OpenELF(path)
	if err != nil {
		return err
	}
	defer elfFile.Close()

	artifactType := buildid.ArtifactType(path, elfFile)
	row := storage.ArtifactInput{
		BuildID:     result.Value,
		Type:        artifactType,
		FilePath:    path,
		ArchivePath: archivePath,
		MemberPath:  memberPath,
		BuildIDKind: string(result.Kind),
		RawBuildID:  result.Raw,
	}
	if err := i.storage.AddArtifact(row, mtimeNS); err != nil {
		return err
	}

	return i.indexSourcesFromDWARF(result.Value, elfFile, path, mtimeNS)
}

func (i *Indexer) indexSourcesFromDWARF(buildID string, f *elf.File, elfPath string, mtimeNS int64) error {
	data, err := f.DWARF()
	if err != nil {
		return nil
	}

	seen := make(map[string]struct{})
	reader := data.Reader()
	for {
		entry, err := reader.Next()
		if err != nil {
			return err
		}
		if entry == nil {
			break
		}
		if entry.Tag != dwarf.TagCompileUnit {
			continue
		}

		compDir := attrString(entry, dwarf.AttrCompDir)
		name := attrString(entry, dwarf.AttrName)
		if name == "" {
			continue
		}

		sourcePath := resolveSourcePath(compDir, name)
		if sourcePath == "" {
			continue
		}
		if _, ok := seen[sourcePath]; ok {
			continue
		}
		seen[sourcePath] = struct{}{}

		filePath := findSourceOnDisk(sourcePath, elfPath)
		if filePath == "" {
			continue
		}

		if err := i.storage.AddSource(buildID, sourcePath, filePath, mtimeNS); err != nil {
			log.Printf("не удалось сохранить source %s: %v", sourcePath, err)
		}
	}
	return nil
}

func attrString(entry *dwarf.Entry, attr dwarf.Attr) string {
	val, ok := entry.Val(attr).(string)
	if !ok {
		return ""
	}
	return val
}

func resolveSourcePath(compDir, name string) string {
	name = filepath.ToSlash(name)
	if filepath.IsAbs(name) {
		return name
	}
	if compDir == "" {
		return ""
	}
	compDir = filepath.ToSlash(compDir)
	if !strings.HasSuffix(compDir, "/") {
		compDir += "/"
	}
	return compDir + name
}

func findSourceOnDisk(sourcePath, elfPath string) string {
	candidates := []string{sourcePath}
	if !filepath.IsAbs(sourcePath) {
		candidates = append(candidates, filepath.Join(filepath.Dir(elfPath), sourcePath))
	}

	for _, candidate := range candidates {
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			abs, err := filepath.Abs(candidate)
			if err == nil {
				return abs
			}
			return candidate
		}
	}
	return ""
}

// IndexELFReader индексирует ELF из потока (для тестов).
func (i *Indexer) IndexELFReader(r io.Reader, mtimeNS int64) error {
	tmp, err := os.CreateTemp(i.cacheDir, "elf-*")
	if err != nil {
		return err
	}
	path := tmp.Name()
	defer os.Remove(path)

	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return i.indexELF(path, mtimeNS, "", "")
}
