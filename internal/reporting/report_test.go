package reporting

import (
	"encoding/json"
	"github.com/naroSEC/DiscoveryX/internal/model"
	"github.com/naroSEC/DiscoveryX/internal/storage"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHTMLIsOfflineAndEscapesCollectedContent(t *testing.T) {
	dir := t.TempDir()
	attack := `</script><img src=x onerror="window.reportXSS=true"><script>alert(1)</script>`
	s := Snapshot{Target: attack, State: "Partial", Data: model.Data{WebServices: []model.WebService{{URL: "javascript:alert(1)", Title: attack}}, Vulnerabilities: []model.Vulnerability{{Name: attack}}}, Commands: []model.Command{{Executable: "C:\\한글 경로\\httpx.exe", Args: []string{"-u", attack}, Platform: "windows"}}}
	if err := Write(dir, s); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "report.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(b)
	if strings.Contains(html, attack) || strings.Contains(html, `<img src=x`) {
		t.Fatal("collected HTML was injected into report markup")
	}
	if !strings.Contains(html, `\u003c/script\u003e`) {
		t.Fatal("JSON script delimiters not escaped")
	}
	if !strings.Contains(html, "Content-Security-Policy") || !strings.Contains(html, "connect-src &#39;none&#39;") {
		t.Fatal("offline content security policy missing")
	}
	if strings.Contains(html, "<script src=") || strings.Contains(html, "<link ") {
		t.Fatal("report depends on external assets")
	}
	if strings.Contains(html, ".innerHTML") || strings.Contains(html, "eval(") {
		t.Fatal("unsafe DOM rendering")
	}
	start := strings.Index(html, `<script id="report-data" type="application/json">`)
	start += len(`<script id="report-data" type="application/json">`)
	end := strings.Index(html[start:], "</script>")
	var decoded Snapshot
	if err = json.Unmarshal([]byte(html[start:start+end]), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Data.WebServices[0].Title != attack || decoded.Commands[0].Args[1] != attack {
		t.Fatal("escaping changed the collected values")
	}
}

func TestRegenerateLegacyRunDoesNotInventCommands(t *testing.T) {
	dir := t.TempDir()
	meta := map[string]any{"config": map[string]any{"Target": "fixture.test"}, "state": "Completed", "started": time.Now().UTC()}
	if err := storage.JSON(filepath.Join(dir, "metadata.json"), meta); err != nil {
		t.Fatal(err)
	}
	if err := storage.JSON(filepath.Join(dir, "status.json"), map[string]any{"data": model.Data{Assets: []model.Asset{{Hostname: "fixture.test"}}}, "stages": []model.StageState{{Name: "httpx", Status: "Completed"}}}); err != nil {
		t.Fatal(err)
	}
	path, err := Regenerate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(path); err != nil {
		t.Fatal(err)
	}
	s, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.Target != "fixture.test" || len(s.Commands) != 0 || len(s.Invocation) != 0 {
		t.Fatal("legacy report inferred commands")
	}
	if len(s.Data.Assets) != 1 {
		t.Fatal(s)
	}
}
