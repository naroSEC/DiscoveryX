package updater

import (
	"discoveryx/internal/storage"
	"os"
	"path/filepath"
	"time"
)

var Repositories = map[string]string{"naabu": "projectdiscovery/naabu", "subfinder": "projectdiscovery/subfinder", "dnsx": "projectdiscovery/dnsx", "httpx": "projectdiscovery/httpx", "katana": "projectdiscovery/katana", "feroxbuster": "epi052/feroxbuster", "nuclei": "projectdiscovery/nuclei"}
var Names = []string{"naabu", "subfinder", "dnsx", "httpx", "katana", "feroxbuster", "nuclei"}

type Installation struct {
	Name         string    `json:"tool_name"`
	Repository   string    `json:"github_repository"`
	Version      string    `json:"installed_version"`
	Tag          string    `json:"release_tag"`
	Platform     string    `json:"platform"`
	Architecture string    `json:"architecture"`
	Path         string    `json:"executable_path"`
	SHA256       string    `json:"binary_sha256"`
	Installed    time.Time `json:"installation_date"`
	Verification string    `json:"verification_status"`
}
type Manifest map[string]Installation

func ReadManifest(dir string) (Manifest, error) {
	m := Manifest{}
	e := storage.ReadJSON(filepath.Join(dir, "manifest.json"), &m)
	if os.IsNotExist(e) {
		e = nil
	}
	return m, e
}
