package engine

import (
	"github.com/naroSEC/DiscoveryX/internal/model"
	"path/filepath"
)

func (e *Engine) recordCommand(command model.Command) error {
	// Store log references relative to the run so a report folder can be moved.
	for _, p := range []*string{&command.Stdout, &command.Stderr} {
		if *p != "" {
			if rel, err := filepath.Rel(e.Dir, *p); err == nil {
				*p = filepath.ToSlash(rel)
			}
		}
	}
	found := false
	for i := range e.State.Commands {
		if e.State.Commands[i].ID == command.ID {
			e.State.Commands[i] = command
			found = true
			break
		}
	}
	if !found {
		e.State.Commands = append(e.State.Commands, command)
	}
	return e.save()
}
