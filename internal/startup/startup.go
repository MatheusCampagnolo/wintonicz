// Package startup lists programs configured to start with Windows or at sign-in.
package startup

// Entry is one startup item.
type Entry struct {
	Name     string `json:"name"`
	Command  string `json:"command"`
	Location string `json:"location"`
	Scope    string `json:"scope"` // user or machine
}

// Source provides startup items.
type Source interface {
	ListStartup() ([]Entry, error)
}
