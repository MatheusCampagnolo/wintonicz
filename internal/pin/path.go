// Package pin implements the opt-in "pin" shortcut: copy the executable to a
// per-user folder and add that folder to the user (never machine) PATH.
package pin

import "strings"

func norm(p string) string {
	return strings.ToLower(strings.TrimRight(strings.TrimSpace(p), `\/`))
}

// AddPath appends dir to a semicolon-separated PATH value if it is not already there.
func AddPath(current, dir string) (string, bool) {
	for _, part := range strings.Split(current, ";") {
		if norm(part) == norm(dir) {
			return current, false
		}
	}
	if strings.TrimSpace(current) == "" {
		return dir, true
	}
	return strings.TrimRight(current, ";") + ";" + dir, true
}

// RemovePath removes every entry equal to dir and leaves other entries untouched.
func RemovePath(current, dir string) (string, bool) {
	parts := strings.Split(current, ";")
	kept := make([]string, 0, len(parts))
	changed := false
	for _, part := range parts {
		if norm(part) == norm(dir) {
			changed = true
			continue
		}
		kept = append(kept, part)
	}
	return strings.Join(kept, ";"), changed
}
