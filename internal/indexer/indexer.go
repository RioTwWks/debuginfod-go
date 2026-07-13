package indexer

import (
	"debug/dwarf"
	"debug/elf"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/your-username/debuginfod-go/internal/storage"
	"github.com/your-username/debuginfod-go/pkg/buildid"
)

// Indexer сканирует файловую систему и заполняет SQLite-индекс.
type Indexer struct {
	storage *storage.Storage
	paths   []string
}

// NewIndexer создаёт индексатор для указанных корневых путей.
func NewIndexer(store *storage.Storage, paths []string) *Indexer {
	return &Indexer{storage: store, paths: paths}
}

// Scan обходит все пути и индексирует ELF-файлы и исходники.
func (i *Indexer) Scan() error {
	var indexed int
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

			if buildid.IsELF(path) {
				if err := i.indexELF(path, info.ModTime().UnixNano()); err != nil {
					log.Printf("ошибка индексации ELF %s: %v", path, err)
				} else {
					indexed++
				}
				return nil
			}

			if isSourceFile(path) {
				i.indexLooseSource(path, info.ModTime().UnixNano())
			}
			return nil
		})
		if err != nil {
			log.Printf("ошибка обхода %s: %v", root, err)
		}
	}
	log.Printf("индексация завершена: обработано ELF-файлов %d", indexed)
	return nil
}

func (i *Indexer) indexELF(path string, mtimeNS int64) error {
	id, err := buildid.FromPath(path)
	if err != nil {
		return err
	}
	id = buildid.Normalize(id)

	elfFile, err := buildid.OpenELF(path)
	if err != nil {
		return err
	}
	defer elfFile.Close()

	artifactType := buildid.ArtifactType(path, elfFile)
	if err := i.storage.AddArtifact(id, path, artifactType, mtimeNS); err != nil {
		return err
	}

	return i.indexSourcesFromDWARF(id, elfFile, path, mtimeNS)
}

func (i *Indexer) indexSourcesFromDWARF(buildID string, f *elf.File, elfPath string, mtimeNS int64) error {
	data, err := f.DWARF()
	if err != nil {
		// Нет DWARF — это нормально для stripped-бинарников.
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

func (i *Indexer) indexLooseSource(path string, mtimeNS int64) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return
	}
	// Свободные исходники без DWARF привязываем к пустому build-id нельзя —
	// они будут найдены только через DWARF-ссылки при индексации ELF.
	_ = abs
	_ = mtimeNS
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

func isSourceFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".c", ".cc", ".cpp", ".cxx", ".h", ".hpp", ".rs", ".go", ".s":
		return true
	default:
		return false
	}
}
