package tui

import tea "charm.land/bubbletea/v2"

func editTextDraft(draft string, message tea.KeyPressMsg, limit int) string {
	switch message.String() {
	case "backspace":
		runes := []rune(draft)
		if len(runes) > 0 {
			return string(runes[:len(runes)-1])
		}
	case "ctrl+u":
		return ""
	default:
		if message.Text != "" {
			draftRunes := []rune(draft)
			remaining := limit - len(draftRunes)
			if remaining > 0 {
				textRunes := []rune(message.Text)
				if len(textRunes) > remaining {
					textRunes = textRunes[:remaining]
				}
				return string(draftRunes) + string(textRunes)
			}
		}
	}
	return draft
}
