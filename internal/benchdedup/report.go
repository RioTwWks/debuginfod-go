package benchdedup

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"
)

// WriteReport сохраняет отчёт в JSON, CSV или текст.
func WriteReport(w io.Writer, report *RunReport, format string) error {
	switch strings.ToLower(format) {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	case "csv":
		return writeCSV(w, report)
	default:
		return writeText(w, report)
	}
}

func writeText(w io.Writer, report *RunReport) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	write := func(format string, args ...any) error {
		_, err := fmt.Fprintf(tw, format, args...)
		return err
	}
	writeln := func(args ...any) error {
		_, err := fmt.Fprintln(tw, args...)
		return err
	}

	if err := write("Strategy A benchmark\t%s\n\n", report.GeneratedAt.Format(time.RFC3339)); err != nil {
		return err
	}

	if len(report.Tools) > 0 {
		if err := writeln("Tools:"); err != nil {
			return err
		}
		for name, ok := range report.Tools {
			status := "missing"
			if ok {
				status = "ok"
			}
			if err := write("  %s\t%s\n", name, status); err != nil {
				return err
			}
		}
		if err := writeln(); err != nil {
			return err
		}
	}

	for _, sc := range report.Scenarios {
		if sc.Skipped != "" {
			if err := write("== %s + %s ==\tSKIPPED (%s)\n\n", sc.Algo, sc.Preprocess, sc.Skipped); err != nil {
				return err
			}
			continue
		}
		post := ""
		if sc.PostCompress {
			post = " + objcopy-zstd(base)"
		}
		if err := write("== %s + %s%s ==\n", sc.Algo, sc.Preprocess, post); err != nil {
			return err
		}
		s := sc.Summary
		rows := []struct {
			format string
			args   []any
		}{
			{"groups\t%d\n", []any{s.GroupCount}},
			{"files\t%d\n", []any{s.FileCount}},
			{"original\t%s (%d)\n", []any{FormatBytes(s.OriginalTotal), s.OriginalTotal}},
			{"stored\t%s (%d)\n", []any{FormatBytes(s.StoredTotal), s.StoredTotal}},
			{"savings\t%.2f%%\n", []any{s.SavingsPct}},
			{"encode\t%d ms\n", []any{s.EncodeTotalMs}},
			{"decode\t%d ms\n", []any{s.DecodeTotalMs}},
			{"verify_failures\t%d\n", []any{s.VerifyFailures}},
			{"errors\t%d\n", []any{s.ErrorCount}},
		}
		for _, row := range rows {
			if err := write(row.format, row.args...); err != nil {
				return err
			}
		}
		if err := writeln(); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func writeCSV(w io.Writer, report *RunReport) error {
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{
		"algo", "preprocess", "post_compress_base",
		"group_key", "file_count", "original_total", "stored_total", "savings_pct",
		"encode_ms", "decode_ms", "verify_failures", "errors", "skipped",
	})
	for _, sc := range report.Scenarios {
		if sc.Skipped != "" {
			_ = cw.Write([]string{sc.Algo, sc.Preprocess, boolStr(sc.PostCompress), "", "", "", "", "", "", "", "", "", sc.Skipped})
			continue
		}
		for _, g := range sc.Groups {
			var encMs, decMs int64
			verifyFail := 0
			for _, p := range g.Pairs {
				encMs += p.EncodeMs
				decMs += p.DecodeMs
				if !p.VerifyOK {
					verifyFail++
				}
			}
			_ = cw.Write([]string{
				sc.Algo,
				sc.Preprocess,
				boolStr(sc.PostCompress),
				g.GroupKey,
				fmt.Sprintf("%d", g.FileCount),
				fmt.Sprintf("%d", g.OriginalTotal),
				fmt.Sprintf("%d", g.StoredTotal),
				fmt.Sprintf("%.4f", g.SavingsPct),
				fmt.Sprintf("%d", encMs),
				fmt.Sprintf("%d", decMs),
				fmt.Sprintf("%d", verifyFail),
				fmt.Sprintf("%d", len(g.Errors)),
				"",
			})
		}
		s := sc.Summary
		_ = cw.Write([]string{
			sc.Algo,
			sc.Preprocess,
			boolStr(sc.PostCompress),
			"__TOTAL__",
			fmt.Sprintf("%d", s.FileCount),
			fmt.Sprintf("%d", s.OriginalTotal),
			fmt.Sprintf("%d", s.StoredTotal),
			fmt.Sprintf("%.4f", s.SavingsPct),
			fmt.Sprintf("%d", s.EncodeTotalMs),
			fmt.Sprintf("%d", s.DecodeTotalMs),
			fmt.Sprintf("%d", s.VerifyFailures),
			fmt.Sprintf("%d", s.ErrorCount),
			"",
		})
	}
	cw.Flush()
	return cw.Error()
}

func boolStr(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

// WriteReportFile пишет отчёт в файл.
func WriteReportFile(path string, report *RunReport, format string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return WriteReport(f, report, format)
}
