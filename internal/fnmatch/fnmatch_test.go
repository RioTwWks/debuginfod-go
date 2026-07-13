package fnmatch

import "testing"

func TestMatchPathname(t *testing.T) {
	tests := []struct {
		pattern string
		name    string
		want    bool
	}{
		{"/usr/bin/*", "/usr/bin/hello", true},
		{"/usr/bin/*", "/usr/bin/sub/hello", false},
		{"/lib/libc.so.*", "/lib/libc.so.6", true},
		{"/lib/libc.so.*", "/lib/libc.so.6.backup", true},
		{"*.c", "main.c", true},
		{"*.c", "dir/main.c", false},
		{"/bin/ls", "/bin/ls", true},
		{"/bin/l?", "/bin/ls", true},
		{"/bin/l?", "/bin/lss", false},
	}
	for _, tc := range tests {
		got := Match(tc.pattern, tc.name, Pathname)
		if got != tc.want {
			t.Errorf("Match(%q, %q) = %v, want %v", tc.pattern, tc.name, got, tc.want)
		}
	}
}
