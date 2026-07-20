// bench-dedup — офлайн A/B-бенчмарк Strategy A (xdelta3 / bsdiff / HDiffPatch + dwz).
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/your-username/debuginfod-go/internal/benchdedup"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "check-tools" {
		runCheckTools(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "list-groups" {
		runListGroups(os.Args[2:])
		return
	}
	runBenchmark(os.Args[1:])
}

func runCheckTools(args []string) {
	fs := flag.NewFlagSet("check-tools", flag.ExitOnError)
	paths := bindToolPaths(fs)
	fs.Parse(args)

	tools := benchdedup.CheckTools(paths)
	for name, ok := range tools {
		status := "MISSING"
		if ok {
			status = "ok"
		}
		fmt.Printf("%-16s %s\n", name+":", status)
	}
}

func runListGroups(args []string) {
	fs := flag.NewFlagSet("list-groups", flag.ExitOnError)
	scanPath := fs.String("scan-path", "", "корень scan path (DEBUGINFOD_SCAN_PATH)")
	project := fs.String("project", "", "фильтр проекта (можно несколько через запятую)")
	minFiles := fs.Int("min-files", 2, "минимум файлов в группе")
	maxGroups := fs.Int("max-groups", 0, "лимит групп (0 = все)")
	fs.Parse(args)

	if *scanPath == "" {
		fatal("укажите --scan-path")
	}

	files, err := benchdedup.Collect(benchdedup.CollectOptions{
		ScanRoots:     []string{*scanPath},
		ProjectFilter: splitCSV(*project),
	})
	if err != nil {
		fatal(err.Error())
	}
	groups := benchdedup.FilterGroups(benchdedup.GroupByStrategyA(files), *minFiles)
	if *maxGroups > 0 && len(groups) > *maxGroups {
		groups = groups[:*maxGroups]
	}

	var totalFiles, totalBytes int64
	for _, g := range groups {
		var gbytes int64
		for _, f := range g.Files {
			gbytes += f.Size
			totalBytes += f.Size
		}
		totalFiles += int64(len(g.Files))
		fmt.Printf("%s\tfiles=%d\tbytes=%s\n", g.Key.String(), len(g.Files), benchdedup.FormatBytes(gbytes))
	}
	fmt.Printf("\nTOTAL groups=%d files=%d bytes=%s\n", len(groups), totalFiles, benchdedup.FormatBytes(totalBytes))
}

func runBenchmark(args []string) {
	fs := flag.NewFlagSet("bench-dedup", flag.ExitOnError)
	scanPath := fs.String("scan-path", "", "корень scan path")
	project := fs.String("project", "", "фильтр проекта (через запятую)")
	workdir := fs.String("workdir", "", "временный каталог (обязателен)")
	algos := fs.String("algos", "xdelta3,bsdiff,hdiffpatch", "алгоритмы через запятую")
	preprocess := fs.String("preprocess", "none,dwz", "препроцессоры: none,dwz")
	postCompress := fs.Bool("post-compress-base", false, "objcopy --compress-debug-sections=zstd на base после дельт")
	minFiles := fs.Int("min-files", 2, "минимум файлов в группе")
	maxGroups := fs.Int("max-groups", 0, "лимит групп (0 = все)")
	maxFiles := fs.Int("max-files", 0, "лимит .debug при collect (0 = все)")
	keepWorkdir := fs.Bool("keep-workdir", false, "не удалять workdir после прогона")
	format := fs.String("format", "text", "формат отчёта: text|json|csv")
	output := fs.String("output", "", "файл отчёта (по умолчанию stdout)")
	paths := bindToolPaths(fs)
	fs.Parse(args)

	if *scanPath == "" {
		fatal("укажите --scan-path")
	}
	if *workdir == "" {
		fatal("укажите --workdir")
	}

	files, err := benchdedup.Collect(benchdedup.CollectOptions{
		ScanRoots:     []string{*scanPath},
		ProjectFilter: splitCSV(*project),
		MaxFiles:      *maxFiles,
	})
	if err != nil {
		fatal(err.Error())
	}
	groups := benchdedup.FilterGroups(benchdedup.GroupByStrategyA(files), *minFiles)

	algoList := benchdedup.ResolveAlgos(splitCSV(*algos), paths)
	if len(algoList) == 0 {
		fatal("нет распознанных алгоритмов в --algos")
	}
	ppList := benchdedup.ResolvePreprocessors(splitCSV(*preprocess), paths)

	opts := benchdedup.RunOptions{
		WorkDir:          filepath.Clean(*workdir),
		Algos:            algoList,
		Preprocessors:    ppList,
		Groups:           groups,
		PostCompressBase: *postCompress,
		Objcopy:          benchdedup.NewObjcopyZstdPost(paths.Objcopy),
		KeepWorkdir:      *keepWorkdir,
		MaxGroups:        *maxGroups,
	}

	report, err := benchdedup.RunBenchmark(opts)
	if err != nil {
		fatal(err.Error())
	}
	report.ScanInfo = fmt.Sprintf("scan-path=%s project=%q groups=%d files=%d", *scanPath, *project, len(groups), len(files))

	if *output == "" {
		if err := benchdedup.WriteReport(os.Stdout, report, *format); err != nil {
			fatal(err.Error())
		}
		return
	}
	if err := benchdedup.WriteReportFile(*output, report, *format); err != nil {
		fatal(err.Error())
	}
	fmt.Fprintf(os.Stderr, "report written to %s\n", *output)
}

func bindToolPaths(fs *flag.FlagSet) benchdedup.ToolPaths {
	var paths benchdedup.ToolPaths
	fs.StringVar(&paths.Xdelta3, "xdelta3", "", "путь к xdelta3")
	fs.StringVar(&paths.Bsdiff, "bsdiff", "", "путь к bsdiff")
	fs.StringVar(&paths.Bspatch, "bspatch", "", "путь к bspatch")
	fs.StringVar(&paths.Hdiffz, "hdiffz", "", "путь к hdiffz")
	fs.StringVar(&paths.Hpatchz, "hpatchz", "", "путь к hpatchz")
	fs.StringVar(&paths.Dwz, "dwz", "", "путь к dwz")
	fs.StringVar(&paths.Objcopy, "objcopy", "", "путь к objcopy")
	return paths
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func fatal(msg string) {
	fmt.Fprintf(os.Stderr, "bench-dedup: %s\n", msg)
	os.Exit(1)
}
