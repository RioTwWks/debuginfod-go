package benchdedup

import (
	"fmt"
	"path/filepath"
	"time"
)

// RunOptions — параметры бенчмарка Strategy A.
type RunOptions struct {
	WorkDir           string
	Algos             []DiffAlgo
	Preprocessors     []Preprocessor
	Groups            []FileGroup
	PostCompressBase  bool
	Objcopy           *ObjcopyZstdPost
	KeepWorkdir       bool
	MaxGroups         int
}

// PairResult — результат одной пары base→target.
type PairResult struct {
	TargetPath     string `json:"target_path"`
	TargetBuildNum int    `json:"target_build_num"`
	PatchPath      string `json:"patch_path"`
	PatchSize      int64  `json:"patch_size"`
	EncodeMs       int64  `json:"encode_ms"`
	DecodeMs       int64  `json:"decode_ms"`
	VerifyOK       bool   `json:"verify_ok"`
	Error          string `json:"error,omitempty"`
}

// GroupResult — результат по одной группе.
type GroupResult struct {
	GroupKey       string        `json:"group_key"`
	FileCount      int           `json:"file_count"`
	BasePath       string        `json:"base_path"`
	BaseBuildNum   int           `json:"base_build_num"`
	OriginalTotal  int64         `json:"original_total"`
	StoredTotal    int64         `json:"stored_total"`
	SavingsPct     float64       `json:"savings_pct"`
	BaseSize       int64         `json:"base_size"`
	BaseCompressed int64         `json:"base_compressed,omitempty"`
	Pairs          []PairResult  `json:"pairs"`
	Errors         []string      `json:"errors,omitempty"`
}

// ScenarioResult — algo + preprocess.
type ScenarioResult struct {
	Algo        string         `json:"algo"`
	Preprocess  string         `json:"preprocess"`
	PostCompress bool          `json:"post_compress_base,omitempty"`
	Groups      []GroupResult  `json:"groups"`
	Summary     SummaryMetrics `json:"summary"`
	Skipped     string         `json:"skipped,omitempty"`
}

// SummaryMetrics — агрегат по сценарию.
type SummaryMetrics struct {
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

// RunReport — полный отчёт бенчмарка.
type RunReport struct {
	GeneratedAt time.Time        `json:"generated_at"`
	ScanInfo    string           `json:"scan_info,omitempty"`
	Scenarios   []ScenarioResult `json:"scenarios"`
	Tools       map[string]bool  `json:"tools"`
}

// RunBenchmark выполняет все сценарии (algo × preprocess).
func RunBenchmark(opts RunOptions) (*RunReport, error) {
	if opts.WorkDir == "" {
		return nil, fmt.Errorf("workdir is required")
	}
	if err := ensureDir(opts.WorkDir); err != nil {
		return nil, err
	}

	groups := opts.Groups
	if opts.MaxGroups > 0 && len(groups) > opts.MaxGroups {
		groups = groups[:opts.MaxGroups]
	}

	report := &RunReport{
		GeneratedAt: time.Now().UTC(),
		Tools:       map[string]bool{},
	}

	for _, algo := range opts.Algos {
		report.Tools[algo.Name()] = algo.Available()
	}
	for _, pp := range opts.Preprocessors {
		report.Tools["preprocess:"+pp.Name()] = pp.Available()
	}

	for _, algo := range opts.Algos {
		if !algo.Available() {
			report.Scenarios = append(report.Scenarios, ScenarioResult{
				Algo:    algo.Name(),
				Skipped: "tool not available in PATH",
			})
			continue
		}
		for _, pp := range opts.Preprocessors {
			if !pp.Available() {
				report.Scenarios = append(report.Scenarios, ScenarioResult{
					Algo:       algo.Name(),
					Preprocess: pp.Name(),
					Skipped:    "preprocessor not available in PATH",
				})
				continue
			}
			scenario, err := runScenario(opts, algo, pp, groups)
			if err != nil {
				return report, err
			}
			report.Scenarios = append(report.Scenarios, *scenario)
		}
	}
	return report, nil
}

func runScenario(opts RunOptions, algo DiffAlgo, pp Preprocessor, groups []FileGroup) (*ScenarioResult, error) {
	scenarioDir := filepath.Join(opts.WorkDir, safeName(algo.Name(), pp.Name()))
	if !opts.KeepWorkdir {
		defer removeAll(scenarioDir)
	}
	if err := ensureDir(scenarioDir); err != nil {
		return nil, err
	}

	result := &ScenarioResult{
		Algo:         algo.Name(),
		Preprocess:   pp.Name(),
		PostCompress: opts.PostCompressBase,
	}

	var sum SummaryMetrics
	for gi, g := range groups {
		gr, err := benchmarkGroup(scenarioDir, gi, algo, pp, g, opts)
		if err != nil {
			return nil, err
		}
		result.Groups = append(result.Groups, gr)
		sum.GroupCount++
		sum.FileCount += gr.FileCount
		sum.OriginalTotal += gr.OriginalTotal
		sum.StoredTotal += gr.StoredTotal
		for _, p := range gr.Pairs {
			sum.EncodeTotalMs += p.EncodeMs
			sum.DecodeTotalMs += p.DecodeMs
			if !p.VerifyOK {
				sum.VerifyFailures++
			}
			if p.Error != "" {
				sum.ErrorCount++
			}
		}
		sum.ErrorCount += len(gr.Errors)
	}
	sum.SavingsPct = pctSavings(sum.OriginalTotal, sum.StoredTotal)
	result.Summary = sum
	return result, nil
}

func benchmarkGroup(scenarioDir string, groupIndex int, algo DiffAlgo, pp Preprocessor, g FileGroup, opts RunOptions) (GroupResult, error) {
	gr := GroupResult{
		GroupKey:  g.Key.String(),
		FileCount: len(g.Files),
	}
	if len(g.Files) < 2 {
		gr.Errors = append(gr.Errors, "group has fewer than 2 files")
		return gr, nil
	}

	groupDir := filepath.Join(scenarioDir, fmt.Sprintf("group_%03d", groupIndex))
	if err := ensureDir(groupDir); err != nil {
		return gr, err
	}

	paths, err := PrepareGroupFiles(pp, groupDir, g.Files)
	if err != nil {
		gr.Errors = append(gr.Errors, err.Error())
		return gr, nil
	}

	base := g.Files[0]
	basePath := paths[0]
	gr.BasePath = base.Path
	gr.BaseBuildNum = base.FileBuildNum

	baseSize, err := fileSize(basePath)
	if err != nil {
		gr.Errors = append(gr.Errors, err.Error())
		return gr, nil
	}
	gr.BaseSize = baseSize
	gr.StoredTotal = baseSize

	if opts.PostCompressBase && opts.Objcopy != nil && opts.Objcopy.Available() {
		compressed, csize, err := opts.Objcopy.CompressInPlace(groupDir, basePath)
		if err != nil {
			gr.Errors = append(gr.Errors, "objcopy post-compress base: "+err.Error())
		} else {
			gr.BaseCompressed = csize
			gr.StoredTotal = csize
			_ = compressed
		}
	}

	for _, f := range g.Files {
		gr.OriginalTotal += f.Size
	}

	for i := 1; i < len(g.Files); i++ {
		targetPath := paths[i]
		patchPath := TempPatchPath(groupDir, algo.Name(), fmt.Sprintf("t%d", i))

		pair := PairResult{
			TargetPath:     g.Files[i].Path,
			TargetBuildNum: g.Files[i].FileBuildNum,
			PatchPath:      patchPath,
		}

		wantSHA, err := fileSHA256(targetPath)
		if err != nil {
			pair.Error = err.Error()
			gr.Pairs = append(gr.Pairs, pair)
			continue
		}

		encStart := time.Now()
		if err := algo.Encode(basePath, targetPath, patchPath); err != nil {
			pair.Error = err.Error()
			gr.Pairs = append(gr.Pairs, pair)
			continue
		}
		pair.EncodeMs = time.Since(encStart).Milliseconds()

		psize, err := fileSize(patchPath)
		if err != nil {
			pair.Error = err.Error()
			gr.Pairs = append(gr.Pairs, pair)
			continue
		}
		pair.PatchSize = psize
		gr.StoredTotal += psize

		outPath := filepath.Join(groupDir, fmt.Sprintf("restored_%d.debug", i))
		decStart := time.Now()
		if err := algo.Decode(basePath, patchPath, outPath); err != nil {
			pair.Error = err.Error()
			gr.Pairs = append(gr.Pairs, pair)
			continue
		}
		pair.DecodeMs = time.Since(decStart).Milliseconds()

		gotSHA, err := fileSHA256(outPath)
		if err != nil {
			pair.Error = err.Error()
		} else {
			pair.VerifyOK = gotSHA == wantSHA
			if !pair.VerifyOK {
				pair.Error = fmt.Sprintf("sha256 mismatch: got %s want %s", gotSHA, wantSHA)
			}
		}
		RemoveIfExists(outPath)
		gr.Pairs = append(gr.Pairs, pair)
	}

	gr.SavingsPct = pctSavings(gr.OriginalTotal, gr.StoredTotal)
	return gr, nil
}
