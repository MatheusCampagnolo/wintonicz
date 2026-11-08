package network

import (
	"context"
	"net"
	"strings"
	"time"
)

// QuickTargets are the only hosts the quick check contacts. They are shown to the
// user before and after the check.
var QuickTargets = struct{ DNSName, IPv4, WebHost string }{
	DNSName: "example.com",
	IPv4:    "1.1.1.1:443",
	WebHost: "example.com:443",
}

// QuickStep is one stage of the quick internet check.
type QuickStep struct {
	Name       string `json:"name"`
	Target     string `json:"target,omitempty"`
	OK         bool   `json:"ok"`
	Detail     string `json:"detail"`
	DurationMS int64  `json:"durationMs"`
}

// QuickResult is the steps plus a plain-language reading of them.
type QuickResult struct {
	Steps     []QuickStep `json:"steps"`
	Diagnosis string      `json:"diagnosis"`
}

// QuickCheck walks from the local adapter outward: router, DNS, internet by IP,
// then a web host by name. It sends one DNS query and two TCP connections.
func QuickCheck(ctx context.Context, adapters []Adapter) QuickResult {
	var steps []QuickStep

	started := time.Now()
	gateway := ""
	for _, a := range adapters {
		if strings.EqualFold(a.Status, "up") && len(a.Gateways) > 0 {
			gateway = a.Name + " → " + a.Gateways[0]
			break
		}
	}
	steps = append(steps, QuickStep{Name: "Adapter with a gateway", OK: gateway != "", DurationMS: time.Since(started).Milliseconds(),
		Detail: map[bool]string{true: gateway, false: "No adapter is up with a default gateway"}[gateway != ""]})

	started = time.Now()
	lookupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	addrs, err := net.DefaultResolver.LookupHost(lookupCtx, QuickTargets.DNSName)
	cancel()
	dns := QuickStep{Name: "DNS lookup", Target: QuickTargets.DNSName, OK: err == nil && len(addrs) > 0, DurationMS: time.Since(started).Milliseconds()}
	if dns.OK {
		dns.Detail = strings.Join(addrs, ", ")
	} else if err != nil {
		dns.Detail = err.Error()
	} else {
		dns.Detail = "no addresses returned"
	}
	steps = append(steps, dns)

	ip, _ := CheckConnectivity(ctx, QuickTargets.IPv4, 3*time.Second)
	steps = append(steps, tcpStep("Internet by IP address", ip))
	web, _ := CheckConnectivity(ctx, QuickTargets.WebHost, 3*time.Second)
	steps = append(steps, tcpStep("Web by name", web))

	return QuickResult{Steps: steps, Diagnosis: diagnose(steps)}
}

func tcpStep(name string, r ConnectivityResult) QuickStep {
	step := QuickStep{Name: name, Target: r.Target, OK: r.TCPConnected, DurationMS: r.DurationMillis}
	if r.TCPConnected {
		step.Detail = "connected"
	} else {
		step.Detail = r.FailureReason
	}
	return step
}

func diagnose(s []QuickStep) string {
	gateway, dns, ip, web := s[0].OK, s[1].OK, s[2].OK, s[3].OK
	switch {
	case gateway && dns && ip && web:
		return "Everything responded. If a specific site fails, the problem is with that site or your browser."
	case !gateway:
		return "No active network connection with a gateway. Check Wi-Fi/cable, then the adapter status."
	case ip && !dns:
		return "The internet works by IP address but names do not resolve: a DNS problem. Try 'Flush DNS cache' or change the DNS server."
	case !ip && !dns:
		return "Connected to the router but nothing reaches the internet. Restart the router/modem or check with your provider."
	case ip && dns && !web:
		return "DNS and IP work but the web host did not answer on port 443. A firewall, proxy, or the site itself may be blocking it."
	}
	return "Mixed results. See the steps above."
}
