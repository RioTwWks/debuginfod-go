package buildid

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const (
	// NT_GNU_BUILD_ID — тип заметки GNU build-id.
	NT_GNU_BUILD_ID = 3
	// NT_GO_BUILD_ID — тип заметки Go build-id.
	NT_GO_BUILD_ID = 4
)

// Kind описывает происхождение build-id.
type Kind string

const (
	KindGNU Kind = "gnu"
	KindGo  Kind = "go"
)

// Result содержит нормализованный build-id и его тип.
type Result struct {
	Value string
	Kind  Kind
	Raw   string
}

// parseNotes разбирает секцию SHT_NOTE и возвращает GNU или Go build-id.
func parseNotes(data []byte) (Result, error) {
	var gnu, goID Result
	offset := 0

	for offset < len(data) {
		if offset+12 > len(data) {
			break
		}

		namesz := int(le32(data[offset:]))
		descsz := int(le32(data[offset+4:]))
		typ := int(le32(data[offset+8:]))
		offset += 12

		namePad := align4(namesz)
		descPad := align4(descsz)
		if offset+namePad+descsz > len(data) {
			break
		}

		name := strings.TrimRight(string(data[offset:offset+namesz]), "\x00")
		offset += namePad

		desc := data[offset : offset+descsz]
		offset += descPad

		switch {
		case typ == NT_GNU_BUILD_ID && name == "GNU" && len(desc) > 0:
			gnu = Result{
				Value: hex.EncodeToString(desc),
				Kind:  KindGNU,
			}
		case typ == NT_GO_BUILD_ID && name == "Go" && len(desc) > 0:
			raw := string(desc)
			goID = Result{
				Value: GoCanonicalID(raw),
				Kind:  KindGo,
				Raw:   raw,
			}
		}
	}

	if gnu.Value != "" {
		return gnu, nil
	}
	if goID.Value != "" {
		return goID, nil
	}
	return Result{}, ErrNotFound
}

// GoCanonicalID превращает Go build-id в hex SHA-256 для URL debuginfod.
// Go build-id содержит символы вроде "/" и не подходит для путей HTTP.
func GoCanonicalID(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// MatchBuildIDQuery проверяет совпадение запроса с индексированным build-id.
func MatchBuildIDQuery(query, indexedID, rawGoID string) bool {
	query = Normalize(query)
	if query == indexedID {
		return true
	}
	if rawGoID != "" && query == GoCanonicalID(rawGoID) {
		return true
	}
	if rawGoID != "" && query == rawGoID {
		return true
	}
	return false
}

func le32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

func align4(n int) int {
	if n%4 == 0 {
		return n
	}
	return n + (4 - n%4)
}
