package benchdedup

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
)

// WriteMatrixReport сохраняет матрицу в JSON, CSV или текст.
func WriteMatrixReport(w io.Writer, report *MatrixReport, format string) error {
	switch strings.ToLower(format) {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	case "csv":
		return writeMatrixCSV(w, report)
	default:
		return writeMatrixText(w, report)
	}
}

func writeMatrixText(w io.Writer, report *MatrixReport) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintf(tw, "Matrix benchmark\t%s\n\n", report.GeneratedAt.Format("2006-01-02 15:04:05 UTC")); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(tw, "%s\n\n", report.ScanInfo); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(tw, "ID\talgo\tpreprocess\tpost_zstd\tgroup_by\tsavings%%\tstored\tverify_err\terrors\tskipped\n"); err != nil {
		return err
	}
	for _, r := range report.Rows {
		skipped := r.Skipped
		if skipped == "" {
			skipped = "-"
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%v\t%s\t%.2f\t%s\t%d\t%d\t%s\n",
			r.ID, r.Algo, r.Preprocess, r.PostCompress, r.GroupBy,
			r.SavingsPct, FormatBytes(r.StoredTotal), r.VerifyFailures, r.ErrorCount, skipped); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func writeMatrixCSV(w io.Writer, report *MatrixReport) error {
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{
		"id", "algo", "preprocess", "post_compress_base", "group_by",
		"group_count", "file_count", "original_total", "stored_total", "savings_pct",
		"encode_ms", "decode_ms", "verify_failures", "errors", "skipped",
	})
	for _, r := range report.Rows {
		_ = cw.Write([]string{
			r.ID, r.Algo, r.Preprocess, boolStr(r.PostCompress), r.GroupBy,
			fmt.Sprintf("%d", r.GroupCount), fmt.Sprintf("%d", r.FileCount),
			fmt.Sprintf("%d", r.OriginalTotal), fmt.Sprintf("%d", r.StoredTotal),
			fmt.Sprintf("%.4f", r.SavingsPct),
			fmt.Sprintf("%d", r.EncodeTotalMs), fmt.Sprintf("%d", r.DecodeTotalMs),
			fmt.Sprintf("%d", r.VerifyFailures), fmt.Sprintf("%d", r.ErrorCount),
			r.Skipped,
		})
	}
	cw.Flush()
	return cw.Error()
}

// WriteMatrixReportFile пишет матрицу в файл.
func WriteMatrixReportFile(path string, report *MatrixReport, format string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return WriteMatrixReport(f, report, format)
}

// WriteMatrixReportFileWithFormats пишет JSON + CSV + text рядом с base path.
func WriteMatrixReportFileWithFormats(basePath string, report *MatrixReport) error {
	ext := ""
	if i := strings.LastIndex(basePath, "."); i >= 0 {
		ext = basePath[i:]
	}
	stem := strings.TrimSuffix(basePath, ext)
	formats := map[string]string{
		stem + ".json": "json",
		stem + ".csv":  "csv",
		stem + ".txt":  "text",
	}
	for path, format := range formats {
		if err := WriteMatrixReportFile(path, report, format); err != nil {
			return err
		}
	}
	return nil
}
