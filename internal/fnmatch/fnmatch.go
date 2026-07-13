// Package fnmatch реализует shell-style glob как fnmatch(3) с FNM_PATHNAME.
package fnmatch

import "strings"

const (
	// Pathname — '*' и '?' не совпадают с '/'.
	Pathname = 1 << iota
)

// Match проверяет, соответствует ли name шаблону pattern.
// flags: Pathname для поведения FNM_PATHNAME (как в debuginfod metadata glob).
func Match(pattern, name string, flags int) bool {
	pattern = strings.ReplaceAll(pattern, "\\", "/")
	name = strings.ReplaceAll(name, "\\", "/")
	return match(pattern, name, flags)
}

func match(pattern, name string, flags int) bool {
	for len(pattern) > 0 {
		switch pattern[0] {
		case '*':
			if matchStar(pattern[1:], name, flags) {
				return true
			}
			return false
		case '?':
			if name == "" {
				return false
			}
			if flags&Pathname != 0 && name[0] == '/' {
				return false
			}
			pattern = pattern[1:]
			name = name[1:]
		case '[':
			end := strings.IndexByte(pattern, ']')
			if end < 0 {
				return false
			}
			if name == "" {
				return false
			}
			if flags&Pathname != 0 && name[0] == '/' {
				return false
			}
			if !matchClass(pattern[1:end], name[0]) {
				return false
			}
			pattern = pattern[end+1:]
			name = name[1:]
		case '\\':
			if len(pattern) < 2 || name == "" {
				return false
			}
			if pattern[1] != name[0] {
				return false
			}
			pattern = pattern[2:]
			name = name[1:]
		default:
			if name == "" || pattern[0] != name[0] {
				return false
			}
			pattern = pattern[1:]
			name = name[1:]
		}
	}
	return name == ""
}

func matchStar(pattern, name string, flags int) bool {
	if pattern == "" {
		if flags&Pathname != 0 {
			return !strings.Contains(name, "/")
		}
		return true
	}
	for i := 0; i <= len(name); i++ {
		if flags&Pathname != 0 && i > 0 && name[i-1] == '/' {
			break
		}
		if match(pattern, name[i:], flags) {
			return true
		}
	}
	return false
}

func matchClass(class string, ch byte) bool {
	if class == "" {
		return false
	}
	inverted := false
	if class[0] == '!' || class[0] == '^' {
		inverted = true
		class = class[1:]
	}
	matched := false
	for i := 0; i < len(class); {
		lo := class[i]
		hi := lo
		if i+2 < len(class) && class[i+1] == '-' {
			hi = class[i+2]
			i += 3
		} else {
			i++
		}
		if ch >= lo && ch <= hi {
			matched = true
		}
	}
	return matched != inverted
}
