package services

import "strings"

// Entry is a read-only snapshot of a Windows service.
type Entry struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	State       string `json:"state"`
	PID         uint32 `json:"pid,omitempty"`
}

// Source provides Windows service snapshots.
type Source interface {
	ListServices() ([]Entry, error)
}

// KeyService is a service that commonly explains everyday Windows problems.
type KeyService struct {
	Name string
	Why  string
}

// KeyServices is the curated list shown by "key services".
var KeyServices = []KeyService{
	{"Dhcp", "Gets an IP address from the router. Stopped: no network address."},
	{"Dnscache", "Caches DNS lookups. Stopped: slow or failing name lookups."},
	{"EventLog", "Records Windows events. Stopped: logs and many tools fail."},
	{"BITS", "Background downloads used by Windows Update."},
	{"wuauserv", "Windows Update. Stopped: updates cannot install."},
	{"Spooler", "Printing. Stopped: no printing; a common fix is restarting it."},
	{"WinDefend", "Microsoft Defender Antivirus."},
	{"mpssvc", "Windows Defender Firewall."},
	{"W32Time", "Clock sync. Stopped: wrong time can break sign-ins and HTTPS."},
	{"Audiosrv", "Windows Audio. Stopped: no sound."},
	{"LanmanWorkstation", "Network file shares (client)."},
	{"RpcSs", "Remote Procedure Call. Core service many others depend on."},
}

// Key returns the entries for the curated services, in curated order. A key service
// missing from the system is reported with state "not installed".
func Key(all []Entry) []Entry {
	byName := map[string]Entry{}
	for _, e := range all {
		byName[strings.ToLower(e.Name)] = e
	}
	out := make([]Entry, 0, len(KeyServices))
	for _, k := range KeyServices {
		if e, ok := byName[strings.ToLower(k.Name)]; ok {
			out = append(out, e)
		} else {
			out = append(out, Entry{Name: k.Name, DisplayName: k.Name, State: "not installed"})
		}
	}
	return out
}

// WhyKey explains why a service is on the curated list ("" if it is not).
func WhyKey(name string) string {
	for _, k := range KeyServices {
		if strings.EqualFold(k.Name, name) {
			return k.Why
		}
	}
	return ""
}
