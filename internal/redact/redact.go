// Package redact masks personal and network identifiers in text before a report
// leaves the program. It works on the final rendered text, so it also catches
// values that appear inside command lines and paths.
package redact

import (
	"net"
	"regexp"
	"strings"
)

// Options select what to mask.
type Options struct {
	Username bool
	Host     bool
	IP       bool
	MAC      bool
}

// All masks everything.
func All() Options { return Options{Username: true, Host: true, IP: true, MAC: true} }

// None masks nothing.
func None() Options { return Options{} }

// Parse reads a comma list such as "username,host,ip,mac" or "none".
func Parse(list string) (Options, error) {
	var o Options
	list = strings.ToLower(strings.TrimSpace(list))
	if list == "" {
		return All(), nil
	}
	for _, item := range strings.Split(list, ",") {
		switch strings.TrimSpace(item) {
		case "username", "user":
			o.Username = true
		case "host", "hostname", "computer":
			o.Host = true
		case "ip":
			o.IP = true
		case "mac":
			o.MAC = true
		case "all":
			return All(), nil
		case "none", "":
		default:
			return Options{}, &UnknownOptionError{Name: strings.TrimSpace(item)}
		}
	}
	return o, nil
}

// UnknownOptionError names a redaction option that does not exist.
type UnknownOptionError struct{ Name string }

func (e *UnknownOptionError) Error() string {
	return "unknown redaction option \"" + e.Name + "\"; use username, host, ip, mac, all, or none"
}

// Names lists the enabled options.
func (o Options) Names() []string {
	var n []string
	if o.Username {
		n = append(n, "username")
	}
	if o.Host {
		n = append(n, "host")
	}
	if o.IP {
		n = append(n, "ip")
	}
	if o.MAC {
		n = append(n, "mac")
	}
	return n
}

// Counts reports how many values of each kind were masked.
type Counts struct {
	Username int `json:"username"`
	Host     int `json:"host"`
	IP       int `json:"ip"`
	MAC      int `json:"mac"`
}

// Total is the number of replacements.
func (c Counts) Total() int { return c.Username + c.Host + c.IP + c.MAC }

// Redactor applies Options for one machine and user.
type Redactor struct {
	opts     Options
	userRE   *regexp.Regexp
	userPath *regexp.Regexp
	hostRE   *regexp.Regexp
}

var (
	macRE  = regexp.MustCompile(`(?i)\b(?:[0-9a-f]{2}[:-]){5}[0-9a-f]{2}\b`)
	ipv4RE = regexp.MustCompile(`(?:^|[^0-9A-Za-z.])((?:\d{1,3}\.){3}\d{1,3})(?:$|[^0-9A-Za-z.])`)
	ipv6RE = regexp.MustCompile(`(?i)(?:\b(?:[0-9a-f]{1,4}:){7}[0-9a-f]{1,4}\b|(?:[0-9a-f]{1,4}:){1,7}:(?:[0-9a-f]{1,4}(?::[0-9a-f]{1,4}){0,5})?)`)
)

// New builds a Redactor. username and host are the values to mask; names shorter
// than three characters are only masked inside \Users\<name> paths, to avoid
// replacing ordinary words.
func New(opts Options, username, host string) *Redactor {
	r := &Redactor{opts: opts}
	if opts.Username && username != "" {
		quoted := regexp.QuoteMeta(username)
		r.userPath = regexp.MustCompile(`(?i)(\\Users\\)` + quoted + `(\\|$|\b)`)
		if len(username) >= 3 {
			r.userRE = regexp.MustCompile(`(?i)\b` + quoted + `\b`)
		}
	}
	if opts.Host && len(host) >= 3 {
		r.hostRE = regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(host) + `\b`)
	}
	return r
}

// Apply masks the text and counts what it changed.
func (r *Redactor) Apply(s string) (string, Counts) {
	var c Counts
	if r.userPath != nil {
		s = r.userPath.ReplaceAllStringFunc(s, func(m string) string {
			c.Username++
			sub := r.userPath.FindStringSubmatch(m)
			return sub[1] + "<user>" + sub[2]
		})
	}
	if r.userRE != nil {
		s = r.userRE.ReplaceAllStringFunc(s, func(string) string { c.Username++; return "<user>" })
	}
	if r.hostRE != nil {
		s = r.hostRE.ReplaceAllStringFunc(s, func(string) string { c.Host++; return "<host>" })
	}
	if r.opts.MAC {
		s = macRE.ReplaceAllStringFunc(s, func(string) string { c.MAC++; return "<mac>" })
	}
	if r.opts.IP {
		s = replaceIPv4(s, &c)
		s = ipv6RE.ReplaceAllStringFunc(s, func(m string) string {
			if net.ParseIP(m) == nil {
				return m
			}
			c.IP++
			return "<ipv6>"
		})
	}
	return s, c
}

// replaceIPv4 masks valid, non-loopback, non-unspecified IPv4 addresses. Version
// strings such as 10.0.26200.1 are not valid addresses and are left alone.
func replaceIPv4(s string, c *Counts) string {
	var out strings.Builder
	last := 0
	for _, loc := range ipv4RE.FindAllStringSubmatchIndex(s, -1) {
		start, end := loc[2], loc[3]
		ip := net.ParseIP(s[start:end])
		if ip == nil || ip.IsLoopback() || ip.IsUnspecified() {
			continue
		}
		out.WriteString(s[last:start])
		out.WriteString("<ipv4>")
		last = end
		c.IP++
	}
	out.WriteString(s[last:])
	return out.String()
}
