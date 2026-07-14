package buildid

import "testing"

func FuzzParseNotes(f *testing.F) {
	f.Add(gnuNoteBytes())
	f.Add(goNoteBytes())
	f.Add([]byte{0x00, 0x01, 0x02, 0x03})

	f.Fuzz(func(t *testing.T, data []byte) {
		result, err := parseNotes(data)
		if err != nil && err != ErrNotFound {
			t.Fatalf("unexpected error: %v", err)
		}
		if err == nil {
			if result.Value == "" {
				t.Fatal("empty value on success")
			}
			if result.Kind != KindGNU && result.Kind != KindGo {
				t.Fatalf("unexpected kind: %q", result.Kind)
			}
		}
	})
}
