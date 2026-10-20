// Package wordlists bundles the default Feroxbuster dictionary.
package wordlists

import "embed"

//go:embed common.txt
var resources embed.FS

func ReadDefault() ([]byte, error) { return resources.ReadFile("common.txt") }
