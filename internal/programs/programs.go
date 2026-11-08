// Package programs lists installed desktop programs from the Uninstall registry keys.
package programs

// Program is one installed program.
type Program struct {
	Name        string `json:"name"`
	Version     string `json:"version,omitempty"`
	Publisher   string `json:"publisher,omitempty"`
	InstallDate string `json:"installDate,omitempty"`
	SizeKB      uint32 `json:"sizeKb,omitempty"`
	Scope       string `json:"scope"` // user or machine
}

// Source provides installed programs.
type Source interface {
	ListPrograms() ([]Program, error)
}
