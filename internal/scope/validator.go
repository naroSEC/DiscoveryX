package scope

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

var label = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

func Host(input string) (string, error) {
	s := strings.TrimSpace(input)
	if strings.Contains(s, "://") {
		u, e := url.Parse(s)
		if e != nil || u.User != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return "", fmt.Errorf("invalid HTTP target %q", input)
		}
		s = u.Hostname()
	}
	s = strings.TrimSuffix(strings.ToLower(s), ".")
	if net.ParseIP(s) != nil {
		return s, nil
	}
	if len(s) == 0 || len(s) > 253 {
		return "", fmt.Errorf("invalid host %q", input)
	}
	for _, p := range strings.Split(s, ".") {
		if !label.MatchString(p) {
			return "", fmt.Errorf("invalid host %q (use ASCII/punycode domains)", input)
		}
	}
	return s, nil
}

type Validator struct {
	Allowed     []string `json:"allowed"`
	Excluded    []string `json:"excluded"`
	ApprovedIPs []string `json:"approved_ips"`
}

func New(target string, allowed, excluded, ips []string) (Validator, error) {
	h, e := Host(target)
	if e != nil {
		return Validator{}, e
	}
	v := Validator{Allowed: []string{h}, ApprovedIPs: ips}
	for _, x := range allowed {
		h, e = Host(x)
		if e != nil {
			return v, e
		}
		v.Allowed = append(v.Allowed, h)
	}
	for _, x := range excluded {
		h, e = Host(x)
		if e != nil {
			return v, e
		}
		v.Excluded = append(v.Excluded, h)
	}
	for _, ip := range ips {
		if net.ParseIP(ip) == nil {
			return v, fmt.Errorf("approved_ips requires literal IP: %s", ip)
		}
	}
	if !v.Contains(target) {
		return v, fmt.Errorf("target is excluded")
	}
	return v, nil
}
func boundary(h, d string) bool {
	return h == d || (net.ParseIP(d) == nil && strings.HasSuffix(h, "."+d))
}
func (v Validator) Contains(s string) bool {
	h, e := Host(s)
	if e != nil {
		return false
	}
	for _, d := range v.Excluded {
		if boundary(h, d) {
			return false
		}
	}
	for _, d := range v.Allowed {
		if boundary(h, d) {
			return true
		}
	}
	return false
}
func (v Validator) PortIP(ip string) bool {
	for _, p := range v.ApprovedIPs {
		if p == ip {
			return true
		}
	}
	for _, d := range v.Allowed {
		if net.ParseIP(d) != nil && d == ip {
			return true
		}
	}
	return false
}
func (v Validator) Regex() string {
	var parts []string
	for _, d := range v.Allowed {
		parts = append(parts, `(?:[a-zA-Z0-9-]+\.)*`+regexp.QuoteMeta(d))
	}
	return `(?i)^https?://(?:` + strings.Join(parts, "|") + `)(?::[0-9]+)?(?:/|$)`
}
func (v Validator) ExcludeRegex() string {
	var p []string
	for _, d := range v.Excluded {
		p = append(p, `(?:[a-zA-Z0-9-]+\.)*`+regexp.QuoteMeta(d))
	}
	if len(p) == 0 {
		return ""
	}
	return `(?i)^https?://(?:` + strings.Join(p, "|") + `)(?::[0-9]+)?(?:/|$)`
}
func URL(s string) (string, error) {
	u, e := url.Parse(strings.TrimSpace(s))
	if e != nil || u.User != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("invalid HTTP URL %q", s)
	}
	h, e := Host(s)
	if e != nil {
		return "", e
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	_ = h
	u.Fragment = ""
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String(), nil
}
