package engine

import (
	"discoveryx/internal/config"
	"discoveryx/internal/model"
	"discoveryx/internal/scope"
	"discoveryx/internal/updater"
	"time"
)

type Metadata struct {
	Resources       map[string]string               `json:"resource_sha256,omitempty"`
	Schema          int                             `json:"schema"`
	Started         time.Time                       `json:"started"`
	Updated         time.Time                       `json:"updated"`
	Config          config.Config                   `json:"config"`
	Scope           scope.Validator                 `json:"scope"`
	Tools           map[string]updater.Installation `json:"tools"`
	TemplateHash    string                          `json:"template_hash,omitempty"`
	TemplateVersion string                          `json:"template_version,omitempty"`
	State           string                          `json:"state"`
}
type State struct {
	Stages         []model.StageState `json:"stages"`
	Data           model.Data         `json:"data"`
	ScannedIPs     []string           `json:"scanned_ips"`
	ActiveDuration time.Duration      `json:"active_duration_ns"`
}
type Checkpoint struct {
	Data       model.Data `json:"data"`
	ScannedIPs []string   `json:"scanned_ips"`
}
type Summary struct {
	Target         string             `json:"target_domain"`
	Duration       time.Duration      `json:"execution_duration_ns"`
	Subdomains     int                `json:"discovered_subdomains"`
	Resolved       int                `json:"resolved_hosts"`
	WebServices    int                `json:"live_web_services"`
	Ports          int                `json:"open_ports"`
	URLs           int                `json:"crawled_urls"`
	Directories    int                `json:"discovered_directories"`
	Findings       int                `json:"vulnerability_events"`
	UniqueFindings int                `json:"unique_findings"`
	Stages         []model.StageState `json:"stages"`
	Output         string             `json:"output_directory"`
	State          string             `json:"state"`
}
