// Package configs bundles the default configuration and technology rules so
// installations made with go install work outside a source checkout.
package configs

import "embed"

//go:embed config.yaml tech-extensions.yaml
var resources embed.FS

func Read(name string) ([]byte, error) { return resources.ReadFile(name) }
