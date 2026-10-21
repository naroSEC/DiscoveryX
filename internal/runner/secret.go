package runner

import (
	"bytes"
	"io"
	"sync"
)

// SecretWriter redacts a runtime credential even when it spans Write calls.
type SecretWriter struct {
	Writer  io.Writer
	Secret  string
	pending []byte
	mu      sync.Mutex
}

func (w *SecretWriter) Name() string {
	if f, ok := w.Writer.(interface{ Name() string }); ok {
		return f.Name()
	}
	return ""
}
func (w *SecretWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.Secret == "" {
		return w.Writer.Write(p)
	}
	w.pending = append(w.pending, p...)
	for len(w.pending) >= len(w.Secret) {
		idx := bytes.Index(w.pending, []byte(w.Secret))
		if idx >= 0 {
			if _, err := w.Writer.Write(w.pending[:idx]); err != nil {
				return 0, err
			}
			if _, err := io.WriteString(w.Writer, "[REDACTED]"); err != nil {
				return 0, err
			}
			w.pending = w.pending[idx+len(w.Secret):]
		} else {
			n := len(w.pending) - len(w.Secret) + 1
			if _, err := w.Writer.Write(w.pending[:n]); err != nil {
				return 0, err
			}
			w.pending = w.pending[n:]
		}
	}
	return len(p), nil
}
func (w *SecretWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, err := w.Writer.Write(w.pending)
	w.pending = nil
	return err
}

// LockedWriter protects shared console and progress writers across branches.
type LockedWriter struct {
	Writer io.Writer
	mu     sync.Mutex
}

func (w *LockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.Writer.Write(p)
}
