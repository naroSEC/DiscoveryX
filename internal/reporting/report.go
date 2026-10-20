// Package reporting renders a self-contained, offline HTML view of a saved run.
package reporting

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/naroSEC/DiscoveryX/internal/model"
	"github.com/naroSEC/DiscoveryX/internal/storage"
	"github.com/naroSEC/DiscoveryX/internal/updater"
	"html/template"
	"os"
	"path/filepath"
	"time"
)

//go:embed page.html report.css report.js
var assets embed.FS

type Snapshot struct {
	Target     string                          `json:"target"`
	State      string                          `json:"state"`
	Started    time.Time                       `json:"started"`
	Updated    time.Time                       `json:"updated"`
	Duration   time.Duration                   `json:"duration_ns"`
	Output     string                          `json:"output"`
	Platform   string                          `json:"platform,omitempty"`
	Invocation []string                        `json:"invocation,omitempty"`
	Stages     []model.StageState              `json:"stages"`
	Data       model.Data                      `json:"data"`
	Commands   []model.Command                 `json:"commands"`
	Tools      map[string]updater.Installation `json:"tools"`
	Demo       bool                            `json:"demo,omitempty"`
}

func Write(dir string, s Snapshot) error {
	payload, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("report data: %w", err)
	}
	page, err := assets.ReadFile("page.html")
	if err != nil {
		return err
	}
	css, err := assets.ReadFile("report.css")
	if err != nil {
		return err
	}
	js, err := assets.ReadFile("report.js")
	if err != nil {
		return err
	}
	hash := func(b []byte) string {
		sum := sha256.Sum256(b)
		return "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
	}
	policy := "default-src 'none'; script-src " + hash(js) + "; style-src " + hash(css) + "; img-src data:; connect-src 'none'; base-uri 'none'; form-action 'none'"
	t, err := template.New("report").Parse(string(page))
	if err != nil {
		return err
	}
	var out bytes.Buffer
	// JSON is produced with encoding/json's default HTML escaping. In particular,
	// '<' and script-closing tags can never leave the inert JSON script element.
	view := struct {
		Target, Policy  string
		Style           template.CSS
		Script, Payload template.JS
	}{s.Target, policy, template.CSS(css), template.JS(js), template.JS(payload)}
	if err = t.Execute(&out, view); err != nil {
		return err
	}
	return storage.Atomic(filepath.Join(dir, "report.html"), out.Bytes())
}

func Read(dir string) (Snapshot, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Snapshot{}, err
	}
	var meta struct {
		Config           struct{ Target string }
		State            string
		Started, Updated time.Time
		Platform         string
		Command          []string
		Tools            map[string]updater.Installation
	}
	if err = storage.ReadJSON(filepath.Join(abs, "metadata.json"), &meta); err != nil {
		return Snapshot{}, fmt.Errorf("read metadata: %w", err)
	}
	var state struct {
		Stages   []model.StageState `json:"stages"`
		Data     model.Data         `json:"data"`
		Commands []model.Command    `json:"commands"`
		Duration time.Duration      `json:"active_duration_ns"`
	}
	if err = storage.ReadJSON(filepath.Join(abs, "status.json"), &state); err != nil {
		return Snapshot{}, fmt.Errorf("read run state: %w", err)
	}
	if len(state.Commands) == 0 {
		err = storage.ReadJSON(filepath.Join(abs, "logs", "commands.json"), &state.Commands)
		if err != nil && !os.IsNotExist(err) {
			return Snapshot{}, fmt.Errorf("read command history: %w", err)
		}
	}
	return Snapshot{Target: meta.Config.Target, State: meta.State, Started: meta.Started, Updated: meta.Updated, Duration: state.Duration, Output: abs, Platform: meta.Platform, Invocation: meta.Command, Stages: state.Stages, Data: state.Data, Commands: state.Commands, Tools: meta.Tools}, nil
}

func Regenerate(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if st, e := os.Stat(abs); e != nil {
		return "", e
	} else if !st.IsDir() {
		return "", fmt.Errorf("run path is not a directory: %s", abs)
	}
	unlock, err := storage.Lock(abs)
	if err != nil {
		return "", err
	}
	defer unlock()
	s, err := Read(abs)
	if err != nil {
		return "", err
	}
	if err = Write(abs, s); err != nil {
		return "", err
	}
	return filepath.Join(abs, "report.html"), nil
}
