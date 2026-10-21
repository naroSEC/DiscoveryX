package model

import "time"

type Asset struct {
	Hostname     string         `json:"hostname"`
	IPs          []string       `json:"ip_addresses"`
	DNS          map[string]any `json:"dns_records,omitempty"`
	WebServices  []string       `json:"web_services,omitempty"`
	DiscoveredBy string         `json:"discovered_by"`
	Resolved     bool           `json:"resolved"`
}
type Port struct {
	Host     string `json:"host"`
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Service  string `json:"service"`
	Version  string `json:"version"`
	Phase    string `json:"scan_phase"`
	Tool     string `json:"source_tool"`
}
type WebService struct {
	URL          string   `json:"url"`
	Hostname     string   `json:"hostname"`
	IP           string   `json:"ip"`
	Status       int      `json:"status_code"`
	Title        string   `json:"title"`
	Technologies []string `json:"technologies"`
	Server       string   `json:"web_server"`
	CDN          string   `json:"cdn_waf"`
	Tool         string   `json:"source_tool"`
}
type Endpoint struct {
	SourceTools []string            `json:"source_tools"`
	Crawled     bool                `json:"crawled"`
	URL         string              `json:"url"`
	Hostname    string              `json:"hostname"`
	Path        string              `json:"path"`
	Query       map[string][]string `json:"query"`
	Source      string              `json:"source"`
	Method      string              `json:"method,omitempty"`
	Tool        string              `json:"source_tool"`
	Initial     bool                `json:"initial"`
	Directory   bool                `json:"directory"`
}
type Vulnerability struct {
	TemplateID      string   `json:"template_id"`
	Name            string   `json:"name"`
	Severity        string   `json:"severity"`
	CVE             []string `json:"cve"`
	Target          string   `json:"target_url"`
	Matched         string   `json:"matched_target"`
	Matcher         string   `json:"matcher"`
	Evidence        string   `json:"evidence_reference"`
	Time            string   `json:"detection_time"`
	TemplateVersion string   `json:"template_version"`
	Tool            string   `json:"source_tool"`
	DuplicateOf     string   `json:"duplicate_of,omitempty"`
}
type Data struct {
	GitHub          []GitHubFinding `json:"github_findings"`
	Assets          []Asset         `json:"assets"`
	Ports           []Port          `json:"ports"`
	WebServices     []WebService    `json:"webservices"`
	Endpoints       []Endpoint      `json:"endpoints"`
	Vulnerabilities []Vulnerability `json:"vulnerabilities"`
	External        []string        `json:"out_of_scope"`
	InvalidHosts    []string        `json:"invalid_hosts"`
}
type StageState struct {
	Name     string            `json:"name"`
	Status   string            `json:"status"`
	Reason   string            `json:"reason,omitempty"`
	Started  time.Time         `json:"started"`
	Duration time.Duration     `json:"duration_ns"`
	Count    int               `json:"count"`
	Files    map[string]string `json:"sha256,omitempty"`
}
