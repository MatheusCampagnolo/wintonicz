package panels

// Entry is a fixed, reviewed Windows settings or management shortcut.
type Entry struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Source interface {
	ListPanels() []Entry
	OpenPanel(key string) error
}
