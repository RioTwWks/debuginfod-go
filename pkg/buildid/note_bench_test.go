package buildid

import "testing"

func gnuNoteBytes() []byte {
	return []byte{
		0x04, 0x00, 0x00, 0x00,
		0x14, 0x00, 0x00, 0x00,
		0x03, 0x00, 0x00, 0x00,
		'G', 'N', 'U', 0x00,
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10,
		0x11, 0x12, 0x13, 0x14,
	}
}

func goNoteBytes() []byte {
	raw := "abc/def/ghi"
	data := []byte{
		0x04, 0x00, 0x00, 0x00,
		byte(len(raw)), 0x00, 0x00, 0x00,
		0x04, 0x00, 0x00, 0x00,
		'G', 'o', 0x00, 0x00,
	}
	data = append(data, raw...)
	for len(data)%4 != 0 {
		data = append(data, 0)
	}
	return data
}

func BenchmarkParseNotesGNU(b *testing.B) {
	data := gnuNoteBytes()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = parseNotes(data)
	}
}

func BenchmarkParseNotesGo(b *testing.B) {
	data := goNoteBytes()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = parseNotes(data)
	}
}

func BenchmarkGoCanonicalID(b *testing.B) {
	raw := "V3tM5FaSu1fxU2Ua_7Bv/IF91Dv7gL90n1SdU2f6T/-6ccm9oZTO6X6Gb5EMng/-wYdIEKlP6aKUBzRQJ7z"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = GoCanonicalID(raw)
	}
}

func BenchmarkNormalize(b *testing.B) {
	id := "AB-CD-EF-01-23-45-67-89-AB-CD-EF-01-23-45-67-89-AB-CD-EF-01"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Normalize(id)
	}
}
