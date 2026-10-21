package model

// GitHubFinding retains GHRecon schema-v1 provenance without feeding discovered
// addresses into the active scanner pipeline.
type GitHubFinding struct {
	SchemaVersion string           `json:"schema_version"`
	Type          string           `json:"type"`
	Value         string           `json:"value"`
	Provider      string           `json:"provider,omitempty"`
	Confidence    float64          `json:"confidence"`
	Sources       []map[string]any `json:"sources"`
	Repository    map[string]any   `json:"repository_details,omitempty"`
	Attributes    map[string]any   `json:"attributes,omitempty"`
	Reason        string           `json:"reason,omitempty"`
	Domain        string           `json:"target_domain"`
	Evidence      string           `json:"evidence_reference"`
	Tool          string           `json:"source_tool"`
}
