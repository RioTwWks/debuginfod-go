package benchdedup

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/your-username/debuginfod-go/pkg/debugfilename"
	"github.com/your-username/debuginfod-go/pkg/elfcomment"
)

// DebugFile — один .debug из каталога build_*.
type DebugFile struct {
	Project      string
	BuildDir     string
	BuildDirNum  int
	Path         string
	Filename     string
	FileStem     string
	Version      string
	FileBuildNum int
	CommitTag    string
	Size         int64
}

// CollectOptions — параметры обхода scan path.
type CollectOptions struct {
	ScanRoots      []string
	ProjectFilter  []string
	MaxFiles       int
}

// Collect находит .debug в каталогах build_* (как dedup.Discover, без БД).
func Collect(opts CollectOptions) ([]DebugFile, error) {
	allowed := projectFilterSet(opts.ProjectFilter)
	var out []DebugFile

	for _, root := range opts.ScanRoots {
		rootAbs, err := filepath.Abs(root)
		if err != nil {
			return out, fmt.Errorf("abs root %q: %w", root, err)
		}
		files, err := collectUnderRoot(rootAbs, allowed)
		if err != nil {
			return out, err
		}
		out = append(out, files...)
		if opts.MaxFiles > 0 && len(out) >= opts.MaxFiles {
			out = out[:opts.MaxFiles]
			break
		}
	}
	return out, nil
}

func collectUnderRoot(rootAbs string, allowed map[string]struct{}) ([]DebugFile, error) {
	var out []DebugFile
	err := filepath.WalkDir(rootAbs, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasPrefix(name, "build_") {
			return nil
		}

		projectName := projectNameForBuildDir(rootAbs, path)
		if !matchesProjectFilter(projectName, allowed) {
			return filepath.SkipDir
		}

		dirNum, err := debugfilename.ParseBuildDir(name)
		if err != nil {
			return filepath.SkipDir
		}

		files, err := collectDebugFiles(projectName, path, dirNum)
		if err != nil {
			return err
		}
		out = append(out, files...)
		return filepath.SkipDir
	})
	return out, err
}

func collectDebugFiles(projectName, dirPath string, dirNum int) ([]DebugFile, error) {
	var out []DebugFile
	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			if path != dirPath && strings.HasPrefix(d.Name(), "build_") {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".debug") {
			return nil
		}
		if strings.HasSuffix(lower, ".xdelta") || strings.HasSuffix(lower, ".zst") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		meta, err := debugfilename.MetadataFromName(name)
		if err != nil {
			return nil
		}
		tag := elfcomment.FromPathOrEmpty(path)
		out = append(out, DebugFile{
			Project:      projectName,
			BuildDir:     dirPath,
			BuildDirNum:  dirNum,
			Path:         path,
			Filename:     meta.Filename,
			FileStem:     meta.Stem,
			Version:      meta.Version,
			FileBuildNum: meta.BuildNum,
			CommitTag:    tag,
			Size:         info.Size(),
		})
		return nil
	})
	return out, err
}

func projectNameForBuildDir(scanRoot, buildDirPath string) string {
	parent := filepath.Dir(buildDirPath)
	rel, err := filepath.Rel(scanRoot, parent)
	if err != nil || rel == "." {
		return filepath.Base(scanRoot)
	}
	return filepath.ToSlash(rel)
}

func projectFilterSet(projects []string) map[string]struct{} {
	if len(projects) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(projects))
	for _, p := range projects {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out[filepath.ToSlash(p)] = struct{}{}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func matchesProjectFilter(projectName string, allowed map[string]struct{}) bool {
	if allowed == nil {
		return true
	}
	projectName = filepath.ToSlash(projectName)
	_, ok := allowed[projectName]
	return ok
}
