package logging

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseLevel(t *testing.T) {
	if parseLevel("debug") != slog.LevelDebug {
		t.Fatal("debug")
	}
	if parseLevel("INFO") != slog.LevelInfo {
		t.Fatal("info")
	}
}

func TestDailyWriterRotatesByDay(t *testing.T) {
	dir := t.TempDir()
	w, err := newDailyWriter(dir, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if _, err := w.Write([]byte("line1\n")); err != nil {
		t.Fatal(err)
	}

	day := time.Now().UTC().Format("2006-01-02")
	path := filepath.Join(dir, "test-"+day+".log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("line1")) {
		t.Fatalf("content=%q", data)
	}
}

func TestSetupFileDebugConsoleInfo(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	old := slog.Default()
	defer slog.SetDefault(old)

	_, closer := Setup(Options{Level: "info", LogDir: dir})
	if closer == nil {
		t.Fatal("expected file closer")
	}
	defer closer.Close()

	slog.Debug("debug-line", "k", 1)
	slog.Info("info-line", "k", 2)

	day := time.Now().UTC().Format("2006-01-02")
	path := filepath.Join(dir, "debuginfod-"+day+".log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "debug-line") {
		t.Fatalf("file missing debug: %s", text)
	}
	if !strings.Contains(text, "info-line") {
		t.Fatalf("file missing info: %s", text)
	}
	_ = buf
}

func TestSetupWithoutLogDir(t *testing.T) {
	_, closer := Setup(Options{Level: "debug", LogDir: ""})
	if closer != nil {
		t.Fatalf("closer=%v", closer)
	}
	if !DebugEnabled() {
		t.Fatal("expected debug enabled")
	}
}

func TestMultiHandler(t *testing.T) {
	var a, b bytes.Buffer
	ha := slog.NewJSONHandler(&a, handlerOptions(slog.LevelDebug))
	hb := slog.NewJSONHandler(&b, handlerOptions(slog.LevelInfo))
	logger := slog.New(newMultiHandler(ha, hb))
	logger.Debug("x")
	if a.Len() == 0 || b.Len() != 0 {
		t.Fatalf("debug: a=%d b=%d", a.Len(), b.Len())
	}
	logger.Info("y")
	if a.Len() == 0 || b.Len() == 0 {
		t.Fatalf("info: a=%d b=%d", a.Len(), b.Len())
	}
}

func TestWithComponent(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	defer slog.SetDefault(old)

	logger := slog.New(slog.NewJSONHandler(&buf, handlerOptions(slog.LevelDebug)))
	slog.SetDefault(logger)

	WithComponent("indexer").Debug("queued", "path", "/tmp/a")
	if !strings.Contains(buf.String(), `"component":"indexer"`) {
		t.Fatalf("missing component: %s", buf.String())
	}
}

func TestRequestMiddlewareSkipsUI(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	defer slog.SetDefault(old)

	logger := slog.New(slog.NewJSONHandler(&buf, handlerOptions(slog.LevelDebug)))
	slog.SetDefault(logger)

	handler := RequestMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, path := range []string{"/ui/api/stats", "/buildid/deadbeef/debuginfo"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	if strings.Count(buf.String(), `"msg":"request"`) != 1 {
		t.Fatalf("request logs=%q", buf.String())
	}
}

func TestRequestIDFromContext(t *testing.T) {
	ctx := WithContext(contextBackground(), "abc123")
	if got := RequestIDFromContext(ctx); got != "abc123" {
		t.Fatalf("request_id=%q", got)
	}
}

var _ io.Closer = (*dailyWriter)(nil)
