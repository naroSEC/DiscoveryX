package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/mod/semver"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Asset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Digest string `json:"digest"`
	Size   int64  `json:"size"`
}
type Release struct {
	Tag        string  `json:"tag_name"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
}
type GitHub struct {
	Client *http.Client
	Base   string
	Token  string
}

func NewGitHub() *GitHub {
	return &GitHub{Client: &http.Client{Timeout: 2 * time.Minute}, Base: "https://api.github.com"}
}
func (g *GitHub) Get(ctx context.Context, url string, max int64) ([]byte, error) {
	for i := 0; i < 4; i++ {
		req, e := http.NewRequestWithContext(ctx, "GET", url, nil)
		if e != nil {
			return nil, e
		}
		req.Header.Set("User-Agent", "DiscoveryX/0.1")
		if g.Token != "" && strings.HasPrefix(url, g.Base+"/") {
			req.Header.Set("Authorization", "Bearer "+g.Token)
		}
		resp, e := g.Client.Do(req)
		var delay = time.Duration(1<<i) * time.Second
		if e == nil {
			if resp.StatusCode == 200 {
				b, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
				resp.Body.Close()
				if int64(len(b)) > max {
					return nil, fmt.Errorf("download exceeds %d bytes", max)
				}
				return b, err
			}
			status := resp.StatusCode
			if sec, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && sec > 0 {
				delay = time.Duration(sec) * time.Second
			}
			if s := resp.Header.Get("X-RateLimit-Reset"); status == 403 && s != "" {
				if v, err := strconv.ParseInt(s, 10, 64); err == nil {
					delay = time.Until(time.Unix(v, 0))
				}
			}
			resp.Body.Close()
			e = fmt.Errorf("GitHub HTTP %d for %s", status, url)
			if status != 429 && status != 403 && status < 500 {
				return nil, e
			}
		}
		if i == 3 {
			return nil, e
		}
		if delay > 30*time.Second {
			return nil, fmt.Errorf("rate limited; retry later: %w", e)
		}
		if delay < time.Second {
			delay = time.Second
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, fmt.Errorf("request failed")
}
func (g *GitHub) Latest(ctx context.Context, repo string) (Release, error) {
	for page := 1; page <= 5; page++ {
		b, e := g.Get(ctx, fmt.Sprintf("%s/repos/%s/releases?per_page=100&page=%d", g.Base, repo, page), 16<<20)
		if e != nil {
			return Release{}, e
		}
		var rr []Release
		if e = json.Unmarshal(b, &rr); e != nil {
			return Release{}, e
		}
		var best *Release
		for i := range rr {
			r := &rr[i]
			if !r.Draft && !r.Prerelease && (best == nil || Compare(r.Tag, best.Tag) > 0) {
				best = r
			}
		}
		if best != nil {
			return *best, nil
		}
		if len(rr) < 100 {
			break
		}
	}
	return Release{}, fmt.Errorf("no stable release for %s", repo)
}
func Compare(a, b string) int {
	va, vb := a, b
	if !strings.HasPrefix(va, "v") {
		va = "v" + va
	}
	if !strings.HasPrefix(vb, "v") {
		vb = "v" + vb
	}
	if semver.IsValid(va) && semver.IsValid(vb) {
		return semver.Compare(va, vb)
	}
	parse := func(s string) []int {
		var out []int
		for _, p := range strings.FieldsFunc(strings.TrimPrefix(s, "v"), func(r rune) bool { return r < '0' || r > '9' }) {
			n, _ := strconv.Atoi(p)
			out = append(out, n)
		}
		return out
	}
	aa, bb := parse(a), parse(b)
	for i := 0; i < len(aa) || i < len(bb); i++ {
		x, y := 0, 0
		if i < len(aa) {
			x = aa[i]
		}
		if i < len(bb) {
			y = bb[i]
		}
		if x < y {
			return -1
		}
		if x > y {
			return 1
		}
	}
	return 0
}
func SelectAsset(r Release, name, platform, arch string) (Asset, error) {
	var choices []Asset
	for _, a := range r.Assets {
		s := strings.ToLower(a.Name)
		if strings.Contains(s, "debug") {
			continue
		}
		if strings.Contains(s, "checksum") || strings.Contains(s, ".sha") || strings.Contains(s, ".sig") {
			continue
		}
		osOK := strings.Contains(s, platform)
		if platform == "windows" {
			osOK = osOK || strings.Contains(s, "pc-windows")
		}
		archOK := strings.Contains(s, arch)
		if arch == "amd64" {
			archOK = archOK || strings.Contains(s, "x86_64")
		}
		if arch == "arm64" {
			archOK = archOK || strings.Contains(s, "aarch64")
		}
		if !archOK && name == "feroxbuster" && platform == "windows" && arch == "amd64" {
			osOK = false
		}
		if osOK && archOK && (strings.HasSuffix(s, ".zip") || strings.HasSuffix(s, ".tar.gz") || strings.HasSuffix(s, ".exe")) {
			choices = append(choices, a)
		}
	}
	if len(choices) != 1 {
		return Asset{}, fmt.Errorf("%s %s has %d unambiguous supported assets for %s/%s; no source-build fallback", name, r.Tag, len(choices), platform, arch)
	}
	return choices[0], nil
}
