package benchdedup

import (
	"fmt"
	"path/filepath"
	"time"
)

// MatrixScenario — один сценарий в полной матрице сравнения.
type MatrixScenario struct {
	ID           string    `json:"id"`
	Algo         string    `json:"algo"`
	Preprocess   string    `json:"preprocess"`
	PostCompress bool      `json:"post_compress_base"`
	GroupBy      GroupMode `json:"group_by"`
}

// MatrixOptions — параметры полного прогона матрицы.
type MatrixOptions struct {
	WorkDir      string
	ScanPath     string
	Project      string
	Files        []DebugFile
	ToolPaths    ToolPaths
	Scenarios    []MatrixScenario
	KeepWorkdir  bool
}

// MatrixRow — строка сводной таблицы.
type MatrixRow struct {
	ID              string  `json:"id"`
	Algo            string  `json:"algo"`
	Preprocess      string  `json:"preprocess"`
	PostCompress    bool    `json:"post_compress_base"`
	GroupBy         string  `json:"group_by"`
	Skipped         string  `json:"skipped,omitempty"`
	GroupCount      int     `json:"group_count"`
	FileCount       int     `json:"file_count"`
	OriginalTotal   int64   `json:"original_total"`
	StoredTotal     int64   `json:"stored_total"`
	SavingsPct      float64 `json:"savings_pct"`
	EncodeTotalMs   int64   `json:"encode_total_ms"`
	DecodeTotalMs   int64   `json:"decode_total_ms"`
	VerifyFailures  int     `json:"verify_failures"`
	ErrorCount      int     `json:"error_count"`
}

// MatrixReport — результат полной матрицы.
type MatrixReport struct {
	GeneratedAt time.Time   `json:"generated_at"`
	ScanInfo    string      `json:"scan_info"`
	Rows        []MatrixRow `json:"rows"`
}

// DefaultMatrix — полная матрица Strategy A/B на group-by stem.
func DefaultMatrix() []MatrixScenario {
	algos := []string{"xdelta3", "bsdiff", "hdiffpatch"}
	preprocess := []string{"none", "dwz", "decompress-dwz"}
	var out []MatrixScenario
	for _, algo := range algos {
		for _, pp := range preprocess {
			out = append(out, MatrixScenario{
				ID:         fmt.Sprintf("%s_%s", algo, pp),
				Algo:       algo,
				Preprocess: pp,
				GroupBy:    GroupModeStem,
			})
		}
	}
	// Strategy B: objcopy zstd на base после дельт (только xdelta3).
	out = append(out,
		MatrixScenario{
			ID:           "xdelta3_none_objcopy",
			Algo:         "xdelta3",
			Preprocess:   "none",
			PostCompress: true,
			GroupBy:      GroupModeStem,
		},
		MatrixScenario{
			ID:           "xdelta3_decompress-dwz_objcopy",
			Algo:         "xdelta3",
			Preprocess:   "decompress-dwz",
			PostCompress: true,
			GroupBy:      GroupModeStem,
		},
	)
	return out
}

// ExtendedMatrix — дополнительно xdelta3 + none по режимам группировки (Strategy A/B/C metadata).
func ExtendedMatrix() []MatrixScenario {
	base := DefaultMatrix()
	extra := []MatrixScenario{
		{ID: "xdelta3_none_stem-version", Algo: "xdelta3", Preprocess: "none", GroupBy: GroupModeStemVersion},
		{ID: "xdelta3_none_strategy-a", Algo: "xdelta3", Preprocess: "none", GroupBy: GroupModeStrategyA},
		{ID: "xdelta3_decompress-dwz_stem-version", Algo: "xdelta3", Preprocess: "decompress-dwz", GroupBy: GroupModeStemVersion},
		{ID: "xdelta3_decompress-dwz_strategy-a", Algo: "xdelta3", Preprocess: "decompress-dwz", GroupBy: GroupModeStrategyA},
	}
	return append(base, extra...)
}

// RunMatrix выполняет все сценарии матрицы.
func RunMatrix(opts MatrixOptions) (*MatrixReport, error) {
	if opts.WorkDir == "" {
		return nil, fmt.Errorf("workdir is required")
	}
	if err := ensureDir(opts.WorkDir); err != nil {
		return nil, err
	}

	report := &MatrixReport{
		GeneratedAt: time.Now().UTC(),
		ScanInfo:    fmt.Sprintf("scan-path=%s project=%q files=%d scenarios=%d", opts.ScanPath, opts.Project, len(opts.Files), len(opts.Scenarios)),
	}

	for _, sc := range opts.Scenarios {
		row := MatrixRow{
			ID:           sc.ID,
			Algo:         sc.Algo,
			Preprocess:   sc.Preprocess,
			PostCompress: sc.PostCompress,
			GroupBy:      string(sc.GroupBy),
		}

		algos := ResolveAlgos([]string{sc.Algo}, opts.ToolPaths)
		if len(algos) == 0 {
			row.Skipped = "unknown algo"
			report.Rows = append(report.Rows, row)
			continue
		}
		if !algos[0].Available() {
			row.Skipped = "tool not available"
			report.Rows = append(report.Rows, row)
			continue
		}

		pps := ResolvePreprocessors([]string{sc.Preprocess}, opts.ToolPaths)
		if len(pps) == 0 || !pps[0].Available() {
			row.Skipped = "preprocessor not available"
			report.Rows = append(report.Rows, row)
			continue
		}

		mode := sc.GroupBy
		if mode == "" {
			mode = GroupModeStem
		}
		groups := FilterGroups(GroupFiles(opts.Files, mode), 2)

		runOpts := RunOptions{
			WorkDir:          filepath.Join(opts.WorkDir, sc.ID),
			Algos:            algos,
			Preprocessors:    pps,
			Groups:           groups,
			PostCompressBase: sc.PostCompress,
			Objcopy:          NewObjcopyZstdPost(opts.ToolPaths.Objcopy),
			KeepWorkdir:      opts.KeepWorkdir,
		}

		result, err := RunBenchmark(runOpts)
		if err != nil {
			return report, fmt.Errorf("scenario %s: %w", sc.ID, err)
		}
		if len(result.Scenarios) == 0 {
			row.Skipped = "no result"
			report.Rows = append(report.Rows, row)
			continue
		}
		s := result.Scenarios[0]
		if s.Skipped != "" {
			row.Skipped = s.Skipped
			report.Rows = append(report.Rows, row)
			continue
		}
		row.GroupCount = s.Summary.GroupCount
		row.FileCount = s.Summary.FileCount
		row.OriginalTotal = s.Summary.OriginalTotal
		row.StoredTotal = s.Summary.StoredTotal
		row.SavingsPct = s.Summary.SavingsPct
		row.EncodeTotalMs = s.Summary.EncodeTotalMs
		row.DecodeTotalMs = s.Summary.DecodeTotalMs
		row.VerifyFailures = s.Summary.VerifyFailures
		row.ErrorCount = s.Summary.ErrorCount
		report.Rows = append(report.Rows, row)
	}
	return report, nil
}
