package webapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/your-username/debuginfod-go/internal/federation"
	"github.com/your-username/debuginfod-go/internal/indexer"
	"github.com/your-username/debuginfod-go/internal/metrics"
	"github.com/your-username/debuginfod-go/internal/storage"
	"github.com/your-username/debuginfod-go/pkg/buildid"
)

func testOpts(store *storage.Storage) ServerOpts {
	return ServerOpts{
		Store:            store,
		MetadataMaxTime:  5 * time.Second,
		MetadataPageSize: 100,
		Metrics:          metrics.New(),
		CacheBytes:       func() int64 { return 0 },
	}
}

func TestHandlerDebugInfoAndExecutable(t *testing.T) {
	tmp := t.TempDir()
	store, err := storage.New(filepath.Join(tmp, "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	execPath := filepath.Join(tmp, "bin")
	debugPath := filepath.Join(tmp, "bin.debug")
	_ = os.WriteFile(execPath, []byte("exec"), 0o644)
	_ = os.WriteFile(debugPath, []byte("debug"), 0o644)

	_ = store.AddArtifact(storage.ArtifactInput{BuildID: "deadbeef", Type: "executable", FilePath: execPath}, 1)
	_ = store.AddArtifact(storage.ArtifactInput{BuildID: "deadbeef", Type: "debuginfo", FilePath: debugPath}, 1)

	handler := NewHandler(testOpts(store))

	req := httptest.NewRequest(http.MethodGet, "/buildid/deadbeef/executable", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("executable status = %d", rec.Code)
	}
}

func TestHandlerSection(t *testing.T) {
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc not available")
	}

	tmp := t.TempDir()
	src := filepath.Join(tmp, "main.c")
	bin := filepath.Join(tmp, "hello")
	_ = os.WriteFile(src, []byte("int main(){return 0;}"), 0o644)
	_ = exec.Command("gcc", "-g", "-o", bin, src).Run()

	store, err := storage.New(filepath.Join(tmp, "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	id, _ := buildid.FromPath(bin)
	id = buildid.Normalize(id)
	_ = store.AddArtifact(storage.ArtifactInput{BuildID: id, Type: "executable", FilePath: bin}, 1)

	handler := NewHandler(testOpts(store))
	req := httptest.NewRequest(http.MethodGet, "/buildid/"+id+"/section/.note.gnu.build-id", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("section status = %d", rec.Code)
	}
}

func TestMetadataHandler(t *testing.T) {
	store, err := storage.New(filepath.Join(t.TempDir(), "meta.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	_ = store.AddArtifact(storage.ArtifactInput{BuildID: "abc", Type: "executable", FilePath: "/opt/bin/tool"}, 1)

	handler := MetadataHandler(testOpts(store))
	req := httptest.NewRequest(http.MethodGet, "/metadata?key=glob&value=/opt/bin/*", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestMetadataHandlerPagination(t *testing.T) {
	store, err := storage.New(filepath.Join(t.TempDir(), "meta-page.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	for _, id := range []string{"a1", "a2", "a3"} {
		_ = store.AddArtifact(storage.ArtifactInput{
			BuildID: id, Type: "executable", FilePath: "/bin/" + id,
		}, 1)
	}

	handler := MetadataHandler(testOpts(store))
	req := httptest.NewRequest(http.MethodGet, "/metadata?key=glob&value=/bin/*&offset=0&limit=2", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	var resp storage.MetadataResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 2 || resp.Complete || resp.NextOffset != 2 {
		t.Fatalf("resp=%+v", resp)
	}
}

func TestZabbixHandler(t *testing.T) {
	store, err := storage.New(filepath.Join(t.TempDir(), "z.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	collector := metrics.New()
	handler := metrics.Handler(collector, store, func() int64 { return 1024 }, "")
	req := httptest.NewRequest(http.MethodGet, "/zabbix", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}

	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["artifacts_total"]; !ok {
		t.Fatalf("missing artifacts_total: %+v", payload)
	}
}

func TestIntegrationHTTPEndpoints(t *testing.T) {
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc not available")
	}

	tmp := t.TempDir()
	src := filepath.Join(tmp, "main.c")
	bin := filepath.Join(tmp, "hello")
	_ = os.WriteFile(src, []byte("int main(){return 0;}"), 0o644)
	_ = exec.Command("gcc", "-g", "-o", bin, src).Run()

	store, err := storage.New(filepath.Join(tmp, "index.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	idx := indexer.NewIndexer(indexer.Options{
		Storage:  store,
		Paths:    []string{tmp},
		CacheDir: filepath.Join(tmp, "cache"),
		Workers:  2,
		Metrics:  metrics.New(),
	})
	if err := idx.Scan(); err != nil {
		t.Fatal(err)
	}

	id, _ := buildid.FromPath(bin)
	id = buildid.Normalize(id)

	server := httptest.NewServer(NewMux(testOpts(store)))
	defer server.Close()

	resp, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz=%d", resp.StatusCode)
	}

	resp, err = http.Get(server.URL + "/buildid/" + id + "/executable")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("executable=%d", resp.StatusCode)
	}

	resp, err = http.Get(server.URL + "/zabbix")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("zabbix=%d", resp.StatusCode)
	}
}

func TestFederationFallback(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("fed-data"))
	}))
	defer upstream.Close()

	store, err := storage.New(filepath.Join(t.TempDir(), "f.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	opts := testOpts(store)
	opts.Federation = federation.New([]string{upstream.URL}, time.Second)
	handler := NewHandler(opts)

	req := httptest.NewRequest(http.MethodGet, "/buildid/unknown/executable", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
