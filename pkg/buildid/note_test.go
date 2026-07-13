package buildid

import (
	"testing"
)

func TestParseNotesGNU(t *testing.T) {
	// namesz=4, descsz=3, type=3 (NT_GNU_BUILD_ID), name="GNU\0", desc=0x010203
	data := []byte{
		0x04, 0x00, 0x00, 0x00,
		0x03, 0x00, 0x00, 0x00,
		0x03, 0x00, 0x00, 0x00,
		'G', 'N', 'U', 0x00,
		0x01, 0x02, 0x03,
	}
	result, err := parseNotes(data)
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != KindGNU {
		t.Fatalf("kind=%q", result.Kind)
	}
	if result.Value != "010203" {
		t.Fatalf("value=%q", result.Value)
	}
}

func TestParseNotesGo(t *testing.T) {
	raw := "abc/def"
	data := []byte{
		0x04, 0x00, 0x00, 0x00,
		byte(len(raw)), 0x00, 0x00, 0x00,
		0x04, 0x00, 0x00, 0x00,
		'G', 'o', 0x00, 0x00,
	}
	data = append(data, raw...)
	// pad desc to 4
	for len(data)%4 != 0 {
		data = append(data, 0)
	}

	result, err := parseNotes(data)
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != KindGo {
		t.Fatalf("kind=%q", result.Kind)
	}
	if result.Raw != raw {
		t.Fatalf("raw=%q", result.Raw)
	}
}
