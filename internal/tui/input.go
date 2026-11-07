package tui

import tea "charm.land/bubbletea/v2"

func editTextDraft(draft string, cursor int, message tea.KeyPressMsg, limit int) (string, int) {
	runes := []rune(draft)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}

	switch message.String() {
	case "ctrl+u":
		return "", 0
	case "ctrl+a", "home":
		return draft, 0
	case "ctrl+e", "end":
		return draft, len(runes)
	case "left":
		return draft, max(0, cursor-1)
	case "right":
		return draft, min(len(runes), cursor+1)
	case "backspace", "ctrl+h":
		return deleteBeforeCursor(runes, cursor)
	case "delete", "ctrl+d":
		return deleteAtCursor(runes, cursor)
	default:
		key := message.Key()
		switch key.Code {
		case tea.KeyHome:
			return draft, 0
		case tea.KeyEnd:
			return draft, len(runes)
		case tea.KeyLeft:
			return draft, max(0, cursor-1)
		case tea.KeyRight:
			return draft, min(len(runes), cursor+1)
		case tea.KeyBackspace, '\b':
			return deleteBeforeCursor(runes, cursor)
		case tea.KeyDelete:
			return deleteAtCursor(runes, cursor)
		}
		if message.Text == "" {
			return draft, cursor
		}
		remaining := limit - len(runes)
		if remaining <= 0 {
			return draft, cursor
		}
		insert := []rune(message.Text)
		if len(insert) > remaining {
			insert = insert[:remaining]
		}
		next := make([]rune, 0, len(runes)+len(insert))
		next = append(next, runes[:cursor]...)
		next = append(next, insert...)
		next = append(next, runes[cursor:]...)
		return string(next), cursor + len(insert)
	}
}

func deleteBeforeCursor(runes []rune, cursor int) (string, int) {
	if cursor == 0 {
		return string(runes), 0
	}
	remaining := append(runes[:cursor-1], runes[cursor:]...)
	return string(remaining), cursor - 1
}

func deleteAtCursor(runes []rune, cursor int) (string, int) {
	if cursor >= len(runes) {
		return string(runes), cursor
	}
	remaining := append(runes[:cursor], runes[cursor+1:]...)
	return string(remaining), cursor
}

func isBackNavigationKey(message tea.KeyPressMsg) bool {
	key := message.Key()
	return key.Code == tea.KeyEscape || key.Code == tea.KeyEsc || key.Code == tea.KeyBackspace ||
		key.Code == '\b' || key.Code == 0x7f
}
