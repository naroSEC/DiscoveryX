package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"discoveryx/internal/runner"
	"discoveryx/internal/storage"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

type Manager struct {
	Dir      string
	GitHub   *GitHub
	Executor runner.Executor
	Log      io.Writer
}

func New(dir string, log io.Writer) *Manager {
	g := NewGitHub()
	g.Token = os.Getenv("GITHUB_TOKEN")
	return &Manager{Dir: dir, GitHub: g, Executor: runner.OSExecutor{}, Log: log}
}
func (m *Manager) Ensure(ctx context.Context, name string, update bool) (Installation, error) {
	if _, ok := Repositories[name]; !ok {
		return Installation{}, fmt.Errorf("unknown tool %s", name)
	}
	manifest, e := ReadManifest(m.Dir)
	if e != nil {
		return Installation{}, e
	}
	old, installed := manifest[name]
	if installed {
		h, e := storage.Hash(old.Path)
		if e != nil || h != old.SHA256 {
			return old, fmt.Errorf("%s binary integrity mismatch; restore or remove its manifest entry before reinstalling", name)
		}
		if _, e = m.Check(ctx, old.Path, name); e != nil {
			return old, e
		}
		if !update {
			return old, nil
		}
	}
	r, e := m.GitHub.Latest(ctx, Repositories[name])
	if e != nil {
		return old, e
	}
	if installed && Compare(r.Tag, old.Tag) <= 0 {
		return old, nil
	}
	a, e := SelectAsset(r, name, runtime.GOOS, runtime.GOARCH)
	if e != nil {
		return old, e
	}
	b, e := m.GitHub.Get(ctx, a.URL, 256<<20)
	if e != nil {
		return old, e
	}
	verification := "unverified (no release checksum)"
	digest := strings.TrimPrefix(a.Digest, "sha256:")
	if a.Digest != "" && !strings.HasPrefix(a.Digest, "sha256:") {
		return old, fmt.Errorf("unsupported release digest %s", a.Digest)
	}
	if digest == "" {
		for _, ca := range r.Assets {
			lower := strings.ToLower(ca.Name)
			if !strings.Contains(lower, "checksum") && !strings.HasSuffix(lower, ".sha256") {
				continue
			}
			cb, err := m.GitHub.Get(ctx, ca.URL, 8<<20)
			if err != nil {
				return old, err
			}
			for _, ln := range strings.Split(string(cb), "\n") {
				parts := strings.Fields(ln)
				if len(parts) >= 2 && strings.TrimPrefix(parts[len(parts)-1], "*") == a.Name {
					digest = parts[0]
					break
				}
			}
			if digest != "" {
				break
			}
		}
	}
	if digest != "" {
		h := sha256.Sum256(b)
		if !strings.EqualFold(hex.EncodeToString(h[:]), digest) {
			return old, fmt.Errorf("release SHA256 mismatch for %s", a.Name)
		}
		verification = "sha256 verified (official GitHub release metadata/checksum; no independent signature verification)"
	}
	exe := name
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	bin, e := Extract(a.Name, exe, b)
	if e != nil {
		return old, e
	}
	dest := filepath.Join(m.Dir, name)
	if e = os.MkdirAll(dest, 0700); e != nil {
		return old, e
	}
	tmp, e := os.CreateTemp(dest, "candidate-*.exe")
	if e != nil {
		return old, e
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, e = tmp.Write(bin); e == nil {
		e = tmp.Sync()
	}
	tmp.Close()
	if e != nil {
		return old, e
	}
	if e = os.Chmod(tmpPath, 0700); e != nil {
		return old, e
	}
	ver, e := m.Check(ctx, tmpPath, name)
	if e != nil {
		return old, fmt.Errorf("candidate rejected; previous binary retained: %w", e)
	}
	normalized := func(s string) string { return strings.TrimPrefix(strings.TrimSpace(s), "v") }
	if normalized(ver) != normalized(r.Tag) {
		return old, fmt.Errorf("candidate version %s differs from release %s", ver, r.Tag)
	}
	// Version-specific path: the old working executable is never overwritten.
	h := sha256.Sum256(bin)
	hash := hex.EncodeToString(h[:])
	target := filepath.Join(dest, hash[:16]+"-"+exe)
	if _, e = os.Stat(target); e == nil {
		return old, fmt.Errorf("candidate path already exists without matching active manifest; inspect %s", target)
	}
	if e = os.Rename(tmpPath, target); e != nil {
		return old, fmt.Errorf("safe replacement failed; old version retained: %w", e)
	}
	info := Installation{Name: name, Repository: Repositories[name], Version: ver, Tag: r.Tag, Platform: runtime.GOOS, Architecture: runtime.GOARCH, Path: target, SHA256: hash, Installed: time.Now().UTC(), Verification: verification}
	manifest[name] = info
	if e = storage.JSON(filepath.Join(m.Dir, "manifest.json"), manifest); e != nil {
		os.Remove(target)
		return old, fmt.Errorf("manifest commit failed; previous version retained: %w", e)
	}
	if m.Log != nil {
		fmt.Fprintf(m.Log, "%s installed %s: %s\n", name, ver, verification)
	}
	return info, nil
}

var versionRE = regexp.MustCompile(`(?i)\bv?(\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?)\b`)

func (m *Manager) Check(ctx context.Context, exe, name string) (string, error) {
	helpFlag, verFlag := "-h", "-version"
	if name == "feroxbuster" {
		helpFlag = "--help"
		verFlag = "--version"
	}
	help, e := runner.Probe(ctx, m.Executor, exe, helpFlag)
	if e != nil {
		return "", e
	}
	args := []string{verFlag}
	if name != "feroxbuster" {
		if !regexp.MustCompile(`(^|[\s,])-duc([\s,]|$)`).MatchString(help) {
			return "", fmt.Errorf("%s lacks required -duc; refusing unmanaged update checks", name)
		}
		args = append(args, "-duc")
	}
	s, e := runner.Capture(ctx, m.Executor, exe, args...)
	if e != nil {
		return "", e
	}
	v := versionRE.FindString(s)
	if v == "" {
		return "", fmt.Errorf("cannot parse installed %s version: %s", name, s)
	}
	return strings.TrimPrefix(v, "v"), nil
}
func safeEntry(s string) bool {
	if strings.Contains(s, `\`) || strings.Contains(s, ":") || strings.HasPrefix(s, "/") {
		return false
	}
	c := path.Clean(s)
	return c != ".." && !strings.HasPrefix(c, "../")
}
func Extract(asset, exe string, b []byte) ([]byte, error) {
	const max = 256 << 20
	var result []byte
	take := func(name string, r io.Reader, size int64, regular bool) error {
		if !safeEntry(name) {
			return fmt.Errorf("unsafe archive path %q", name)
		}
		if path.Base(name) != exe {
			return nil
		}
		if !regular || size > max || size < 0 {
			return fmt.Errorf("invalid executable entry")
		}
		if result != nil {
			return fmt.Errorf("ambiguous executable entries")
		}
		v, e := io.ReadAll(io.LimitReader(r, max+1))
		if len(v) > max {
			return fmt.Errorf("executable too large")
		}
		result = v
		return e
	}
	switch {
	case strings.HasSuffix(asset, ".zip"):
		z, e := zip.NewReader(bytes.NewReader(b), int64(len(b)))
		if e != nil {
			return nil, e
		}
		for _, f := range z.File {
			if !safeEntry(f.Name) {
				return nil, fmt.Errorf("unsafe archive path %q", f.Name)
			}
			if path.Base(f.Name) != exe {
				continue
			}
			r, e := f.Open()
			if e != nil {
				return nil, e
			}
			e = take(f.Name, r, int64(f.UncompressedSize64), f.Mode().IsRegular())
			r.Close()
			if e != nil {
				return nil, e
			}
		}
	case strings.HasSuffix(asset, ".tar.gz"):
		gz, e := gzip.NewReader(bytes.NewReader(b))
		if e != nil {
			return nil, e
		}
		defer gz.Close()
		tr := tar.NewReader(io.LimitReader(gz, 1<<30))
		for {
			h, e := tr.Next()
			if e == io.EOF {
				break
			}
			if e != nil {
				return nil, e
			}
			if e = take(h.Name, tr, h.Size, h.Typeflag == tar.TypeReg); e != nil {
				return nil, e
			}
		}
	case strings.HasSuffix(asset, ".exe"):
		result = b
	default:
		return nil, fmt.Errorf("unsupported archive %s", asset)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("archive has no %s", exe)
	}
	return result, nil
}
