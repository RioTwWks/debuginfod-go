package benchdedup

import (
	"debug/elf"
	"fmt"
	"os"
	"strings"

	"github.com/your-username/debuginfod-go/pkg/elfcomment"
)

// FileInspect — диагностика одного .debug для сравнения подходов.
type FileInspect struct {
	Path               string   `json:"path"`
	Size               int64    `json:"size"`
	CommentLines       []string `json:"comment_lines"`
	GitCommit          string   `json:"git_commit"`
	CompressedSections []string `json:"compressed_debug_sections"`
	DwzLikely          bool     `json:"dwz_likely"`
	DwzNote            string   `json:"dwz_note"`
}

// InspectFile собирает метаданные ELF для отчёта.
func InspectFile(path string) (FileInspect, error) {
	info := FileInspect{Path: path}
	st, err := os.Stat(path)
	if err != nil {
		return info, err
	}
	info.Size = st.Size()

	f, err := elf.Open(path)
	if err != nil {
		return info, fmt.Errorf("open elf: %w", err)
	}
	defer f.Close()

	if sec := f.Section(".comment"); sec != nil {
		if data, err := sec.Data(); err == nil {
			info.CommentLines = elfcomment.CommentLines(data)
			if commit, err := elfcomment.ParseBytes(data); err == nil {
				info.GitCommit = commit
			}
		}
	}

	for _, sec := range f.Sections {
		if sec == nil || !strings.HasPrefix(sec.Name, ".debug") {
			continue
		}
		if sec.Flags&elf.SHF_COMPRESSED != 0 {
			info.CompressedSections = append(info.CompressedSections, sec.Name)
		}
	}

	info.DwzLikely = len(info.CompressedSections) == 0
	if len(info.CompressedSections) > 0 {
		info.DwzNote = fmt.Sprintf(
			"dwz откажется: сжатые секции (%s); нужен objcopy --decompress-debug-sections",
			strings.Join(info.CompressedSections, ", "),
		)
	} else {
		info.DwzNote = "dwz может работать напрямую"
	}
	return info, nil
}
