// CLI debuginfod-find — обёртка над HTTP API debuginfod-go.
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/your-username/debuginfod-go/pkg/buildid"
)

func main() {
	baseURL := flag.String("url", "", "базовый URL debuginfod (или DEBUGINFOD_URLS)")
	outPath := flag.String("o", "", "сохранить в файл (по умолчанию stdout)")
	metaKey := flag.String("key", "", "metadata key: glob|file|buildid")
	metaValue := flag.String("value", "", "metadata value")
	flag.Parse()

	server := resolveBaseURL(*baseURL)
	if server == "" {
		fatal("укажите --url или DEBUGINFOD_URLS")
	}

	if *metaKey != "" {
		if err := runMetadata(server, *metaKey, *metaValue, *outPath); err != nil {
			fatal(err.Error())
		}
		return
	}

	if flag.NArg() < 2 {
		usage()
	}

	artifactType := strings.ToLower(flag.Arg(0))
	buildID := buildid.Normalize(flag.Arg(1))

	var path string
	switch artifactType {
	case "debuginfo":
		path = fmt.Sprintf("/buildid/%s/debuginfo", buildID)
	case "executable":
		path = fmt.Sprintf("/buildid/%s/executable", buildID)
	case "source":
		if flag.NArg() < 3 {
			fatal("для source укажите путь: debuginfod-find source BUILDID /path/to/file.c")
		}
		src := flag.Arg(2)
		if !strings.HasPrefix(src, "/") {
			src = "/" + src
		}
		path = fmt.Sprintf("/buildid/%s/source%s", buildID, src)
	case "section":
		if flag.NArg() < 3 {
			fatal("для section укажите имя: debuginfod-find section BUILDID .note.gnu.build-id")
		}
		path = fmt.Sprintf("/buildid/%s/section/%s", buildID, flag.Arg(2))
	default:
		fatal("неизвестный тип: " + artifactType)
	}

	if err := download(server+path, *outPath); err != nil {
		fatal(err.Error())
	}
}

func runMetadata(server, key, value, outPath string) error {
	if key == "" || value == "" {
		return fmt.Errorf("metadata требует --key и --value")
	}
	u, err := url.Parse(server + "/metadata")
	if err != nil {
		return err
	}
	q := u.Query()
	q.Set("key", key)
	q.Set("value", value)
	u.RawQuery = q.Encode()

	resp, err := httpGet(u.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("metadata: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if outPath == "" || outPath == "-" {
		_, err = os.Stdout.Write(data)
		return err
	}
	return os.WriteFile(outPath, data, 0o644)
}

func download(rawURL, outPath string) error {
	resp, err := httpGet(rawURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("not found: %s", rawURL)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	if outPath == "" || outPath == "-" {
		_, err = io.Copy(os.Stdout, resp.Body)
		return err
	}

	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	abs, _ := filepath.Abs(outPath)
	fmt.Println(abs)
	return nil
}

func httpGet(rawURL string) (*http.Response, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func resolveBaseURL(flagURL string) string {
	if flagURL != "" {
		return strings.TrimRight(flagURL, "/")
	}
	env := os.Getenv("DEBUGINFOD_URLS")
	if env == "" {
		return ""
	}
	parts := strings.Split(env, ",")
	return strings.TrimSpace(strings.TrimRight(parts[0], "/"))
}

func usage() {
	fmt.Fprintf(os.Stderr, `Использование:
  debuginfod-find [--url URL] debuginfo BUILDID
  debuginfod-find [--url URL] executable BUILDID
  debuginfod-find [--url URL] source BUILDID /path/to/source.c
  debuginfod-find [--url URL] section BUILDID SECTION
  debuginfod-find [--url URL] --key glob --value '/usr/bin/*' [-o file.json]

Переменные окружения:
  DEBUGINFOD_URLS   базовый URL (первый из списка)

Примеры:
  DEBUGINFOD_URLS=http://localhost:8002 debuginfod-find executable deadbeef
  debuginfod-find -o /tmp/dbg --url http://host:8002 debuginfo abcd1234
`)
	os.Exit(2)
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "debuginfod-find:", msg)
	os.Exit(1)
}
