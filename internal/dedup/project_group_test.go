package dedup

import (
	"testing"

	"github.com/your-username/debuginfod-go/internal/storage"
)

func TestNormalizeDedupGroupProject(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Released/QuikServer_16.0_Common_Linux", "Released/QuikServer_16.0_Common_Linux"},
		{"Released/QuikServer_16.0_Release_Linux/16/16.0.0", "Released/QuikServer_16.0_Release_Linux"},
		{"Released/QuikServer_16.0_Release_Linux/16/16.0.7", "Released/QuikServer_16.0_Release_Linux"},
		{"Unsorted/ClientCodeSubstitute_Linux/1/1.6.4", "Unsorted/ClientCodeSubstitute_Linux"},
		{"Released/Quik", "Released/Quik"},
		{"16.0.0", "16.0.0"},
	}
	for _, tc := range cases {
		got := NormalizeDedupGroupProject(tc.in)
		if got != tc.want {
			t.Fatalf("NormalizeDedupGroupProject(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestGroupFilesAcrossVersionPaths(t *testing.T) {
	files := []storage.DedupFile{
		{ProjectName: "Released/Quik/16/16.0.0", FileStem: "libcore.so", FileBuildNum: 1},
		{ProjectName: "Released/Quik/16/16.0.1", FileStem: "libcore.so", FileBuildNum: 2},
		{ProjectName: "Released/Quik/16/16.0.1", FileStem: "libnode.so", FileBuildNum: 3},
	}
	groups := GroupFiles(files)
	if len(groups) != 2 {
		t.Fatalf("groups=%d want 2", len(groups))
	}
	key := groupKeyString(GroupKey(files[0]))
	if len(groups[key]) != 2 {
		t.Fatalf("libcore group size=%d want 2", len(groups[key]))
	}
}
