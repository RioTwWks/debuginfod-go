package benchdedup

import (
	"bytes"
	"testing"
)

func TestGroupByStrategyA(t *testing.T) {
	files := []DebugFile{
		{Project: "P", FileStem: "lib", Version: "1.0", CommitTag: "abc", FileBuildNum: 10, Size: 100},
		{Project: "P", FileStem: "lib", Version: "1.0", CommitTag: "abc", FileBuildNum: 20, Size: 200},
		{Project: "P", FileStem: "lib", Version: "1.0", CommitTag: "xyz", FileBuildNum: 5, Size: 50},
	}
	groups := GroupByStrategyA(files)
	if len(groups) != 2 {
		t.Fatalf("groups=%d want 2", len(groups))
	}
	for _, g := range groups {
		if g.Key.CommitTag == "abc" {
			if len(g.Files) != 2 {
				t.Fatalf("abc group files=%d", len(g.Files))
			}
			if g.Files[0].FileBuildNum != 10 || g.Files[1].FileBuildNum != 20 {
				t.Fatalf("sort order wrong: %+v", g.Files)
			}
		}
	}
}

func TestPctSavings(t *testing.T) {
	got := pctSavings(1000, 800)
	if got < 19.9 || got > 20.1 {
		t.Fatalf("savings=%f want ~20", got)
	}
}

func TestFilterGroups(t *testing.T) {
	groups := []FileGroup{
		{Files: []DebugFile{{}, {}}},
		{Files: []DebugFile{{}}},
	}
	out := FilterGroups(groups, 2)
	if len(out) != 1 {
		t.Fatalf("filtered=%d want 1", len(out))
	}
}

func TestResolveAlgos(t *testing.T) {
	algos := ResolveAlgos([]string{"xdelta3", "bsdiff", "hdiff"}, ToolPaths{})
	if len(algos) != 3 {
		t.Fatalf("algos=%d want 3", len(algos))
	}
}

func TestWriteReportJSON(t *testing.T) {
	report := &RunReport{
		Scenarios: []ScenarioResult{{
			Algo:       "xdelta3",
			Preprocess: "none",
			Summary: SummaryMetrics{
				GroupCount:    1,
				OriginalTotal: 1000,
				StoredTotal:   900,
				SavingsPct:    10,
			},
		}},
	}
	var buf bytes.Buffer
	if err := WriteReport(&buf, report, "json"); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("xdelta3")) {
		t.Fatalf("json=%s", buf.String())
	}
}
