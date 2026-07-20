package benchdedup

import (
	"bytes"
	"testing"
)

func TestDefaultMatrixCount(t *testing.T) {
	m := DefaultMatrix()
	// 3 algos × 3 preprocess + 2 objcopy xdelta
	if len(m) != 11 {
		t.Fatalf("default matrix len=%d want 11", len(m))
	}
}

func TestExtendedMatrixCount(t *testing.T) {
	m := ExtendedMatrix()
	if len(m) != 15 {
		t.Fatalf("extended matrix len=%d want 15", len(m))
	}
}

func TestWriteMatrixReportJSON(t *testing.T) {
	report := &MatrixReport{
		Rows: []MatrixRow{{
			ID: "xdelta3_none", Algo: "xdelta3", Preprocess: "none",
			GroupBy: "stem", SavingsPct: 17.2, StoredTotal: 1000,
		}},
	}
	var buf bytes.Buffer
	if err := WriteMatrixReport(&buf, report, "json"); err != nil {
		t.Fatal(err)
	}
}
