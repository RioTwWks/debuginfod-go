// bench-dedup — офлайн A/B-бенчмарк Strategy A (xdelta3 / bsdiff / HDiffPatch + dwz).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/your-username/debuginfod-go/internal/benchdedup"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "check-tools":
			runCheckTools(os.Args[2:])
			return
		case "list-groups":
			runListGroups(os.Args[2:])
			return
		case "list-files":
			runListFiles(os.Args[2:])
			return
		case "inspect-file":
			runInspectFile(os.Args[2:])
			return
		}
	}
	runBenchmark(os.Args[1:])
}

func parseFlags(fs *flag.FlagSet, args []string) {
	if err := fs.Parse(args); err != nil {
		fatal(err.Error())
	}
}

func bindCollectFlags(fs *flag.FlagSet) (scanPath, project, groupBy *string) {
	scanPath = fs.String("scan-path", "", "корень scan path (DEBUGINFOD_SCAN_PATH)")
	project = fs.String("project", "", "фильтр проекта (можно несколько через запятую)")
	groupBy = fs.String("group-by", "stem", "группировка: stem (default), stem-version, strategy-a")
	return scanPath, project, groupBy
}

func collectFiles(scanPath, project string) ([]benchdedup.DebugFile, error) {
	return benchdedup.Collect(benchdedup.CollectOptions{
		ScanRoots:     []string{scanPath},
		ProjectFilter: splitCSV(project),
	})
}

func resolveGroupMode(groupBy string) benchdedup.GroupMode {
	mode, err := benchdedup.ParseGroupMode(groupBy)
	if err != nil {
		fatal(err.Error())
	}
	return mode
}

func runCheckTools(args []string) {
	fs := flag.NewFlagSet("check-tools", flag.ExitOnError)
	paths := bindToolPaths(fs)
	parseFlags(fs, args)

	tools := benchdedup.CheckTools(paths)
	for name, ok := range tools {
		status := "MISSING"
		if ok {
			status = "ok"
		}
		fmt.Printf("%-16s %s\n", name+":", status)
	}
}

func runListFiles(args []string) {
	fs := flag.NewFlagSet("list-files", flag.ExitOnError)
	scanPath, project, _ := bindCollectFlags(fs)
	maxFiles := fs.Int("max-files", 0, "лимит строк (0 = все)")
	parseFlags(fs, args)

	if *scanPath == "" {
		fatal("укажите --scan-path")
	}

	files, err := collectFiles(*scanPath, *project)
	if err != nil {
		fatal(err.Error())
	}
	if *maxFiles > 0 && len(files) > *maxFiles {
		files = files[:*maxFiles]
	}

	fmt.Printf("path\tstem\tversion\tbuild_num\tgit_commit\tbytes\n")
	for _, f := range files {
		fmt.Printf("%s\t%s\t%s\t%d\t%s\t%d\n",
			f.Path, f.FileStem, f.Version, f.FileBuildNum, f.CommitTag, f.Size)
	}
	fmt.Fprintf(os.Stderr, "\nTOTAL files=%d\n", len(files))
}

func runInspectFile(args []string) {
	fs := flag.NewFlagSet("inspect-file", flag.ExitOnError)
	path := fs.String("path", "", "путь к .debug")
	format := fs.String("format", "text", "text|json")
	parseFlags(fs, args)
	if *path == "" {
		fatal("укажите --path")
	}
	info, err := benchdedup.InspectFile(*path)
	if err != nil {
		fatal(err.Error())
	}
	if strings.ToLower(*format) == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(info); err != nil {
			fatal(err.Error())
		}
		return
	}
	fmt.Printf("path: %s\n", info.Path)
	fmt.Printf("size: %s (%d)\n", benchdedup.FormatBytes(info.Size), info.Size)
	fmt.Printf("git_commit: %s\n", info.GitCommit)
	fmt.Printf("dwz: %s\n", info.DwzNote)
	if len(info.CompressedSections) > 0 {
		fmt.Printf("compressed_debug_sections: %s\n", strings.Join(info.CompressedSections, ", "))
	}
	fmt.Println(".comment:")
	for _, line := range info.CommentLines {
		fmt.Printf("  %s\n", line)
	}
}

func runListGroups(args []string) {
	fs := flag.NewFlagSet("list-groups", flag.ExitOnError)
	scanPath, project, groupBy := bindCollectFlags(fs)
	minFiles := fs.Int("min-files", 2, "минимум файлов в группе")
	maxGroups := fs.Int("max-groups", 0, "лимит групп (0 = все)")
	parseFlags(fs, args)

	if *scanPath == "" {
		fatal("укажите --scan-path")
	}

	files, err := collectFiles(*scanPath, *project)
	if err != nil {
		fatal(err.Error())
	}
	mode := resolveGroupMode(*groupBy)
	stats := benchdedup.ComputeGroupStats(files, mode)
	groups := benchdedup.FilterGroups(benchdedup.GroupFiles(files, mode), *minFiles)
	if *maxGroups > 0 && len(groups) > *maxGroups {
		groups = groups[:*maxGroups]
	}

	_ = printGroupDiagnostics(os.Stderr, stats, *minFiles)

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
	fmt.Printf("\nTOTAL groups=%d files=%d bytes=%s (group-by=%s)\n",
		len(groups), totalFiles, benchdedup.FormatBytes(totalBytes), mode)
}

func printGroupDiagnostics(w io.Writer, stats benchdedup.GroupStats, minFiles int) error {
	if _, err := fmt.Fprintf(w, "collect: files=%d groups=%d groups>=%d=%d singletons=%d largest=%d group-by=%s\n",
		stats.TotalFiles, stats.TotalGroups, minFiles, stats.GroupsGE2, stats.Singletons, stats.LargestGroup, stats.Mode); err != nil {
		return err
	}
	if stats.TotalFiles > 0 && stats.GroupsGE2 == 0 {
		if _, err := fmt.Fprintf(w, "hint: все группы одиночные — попробуйте --group-by stem (как production dedup)\n"); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w)
	return err
}

func runBenchmark(args []string) {
	fs := flag.NewFlagSet("bench-dedup", flag.ExitOnError)
	scanPath, project, groupBy := bindCollectFlags(fs)
	workdir := fs.String("workdir", "", "временный каталог (обязателен)")
	algos := fs.String("algos", "xdelta3,bsdiff,hdiffpatch", "алгоритмы через запятую")
	preprocess := fs.String("preprocess", "none,dwz,decompress-dwz", "препроцессоры: none,dwz,decompress-dwz")
	postCompress := fs.Bool("post-compress-base", false, "objcopy --compress-debug-sections=zstd на base после дельт")
	minFiles := fs.Int("min-files", 2, "минимум файлов в группе")
	maxGroups := fs.Int("max-groups", 0, "лимит групп (0 = все)")
	maxFiles := fs.Int("max-files", 0, "лимит .debug при collect (0 = все)")
	keepWorkdir := fs.Bool("keep-workdir", false, "не удалять workdir после прогона")
	format := fs.String("format", "text", "формат отчёта: text|json|csv")
	output := fs.String("output", "", "файл отчёта (по умолчанию stdout)")
	paths := bindToolPaths(fs)
	parseFlags(fs, args)

	if *scanPath == "" {
		fatal("укажите --scan-path")
	}
	if *workdir == "" {
		fatal("укажите --workdir")
	}

	opts := benchdedup.CollectOptions{
		ScanRoots:     []string{*scanPath},
		ProjectFilter: splitCSV(*project),
		MaxFiles:      *maxFiles,
	}
	files, err := benchdedup.Collect(opts)
	if err != nil {
		fatal(err.Error())
	}
	mode := resolveGroupMode(*groupBy)
	stats := benchdedup.ComputeGroupStats(files, mode)
	_ = printGroupDiagnostics(os.Stderr, stats, *minFiles)
	groups := benchdedup.FilterGroups(benchdedup.GroupFiles(files, mode), *minFiles)

	algoList := benchdedup.ResolveAlgos(splitCSV(*algos), paths)
	if len(algoList) == 0 {
		fatal("нет распознанных алгоритмов в --algos")
	}
	ppList := benchdedup.ResolvePreprocessors(splitCSV(*preprocess), paths)

	runOpts := benchdedup.RunOptions{
		WorkDir:          filepath.Clean(*workdir),
		Algos:            algoList,
		Preprocessors:    ppList,
		Groups:           groups,
		PostCompressBase: *postCompress,
		Objcopy:          benchdedup.NewObjcopyZstdPost(paths.Objcopy),
		KeepWorkdir:      *keepWorkdir,
		MaxGroups:        *maxGroups,
	}

	report, err := benchdedup.RunBenchmark(runOpts)
	if err != nil {
		fatal(err.Error())
	}
	report.ScanInfo = fmt.Sprintf("scan-path=%s project=%q group-by=%s groups=%d files=%d",
		*scanPath, *project, mode, len(groups), len(files))

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
