package pin

import "testing"

func TestAddAndRemovePathAreIdempotentAndCaseInsensitive(t *testing.T) {
	got, changed := AddPath(`C:\Windows;C:\Tools`, `C:\Users\me\AppData\Local\Wintonicz\bin`)
	if !changed || got != `C:\Windows;C:\Tools;C:\Users\me\AppData\Local\Wintonicz\bin` {
		t.Fatalf("AddPath = %q, %v", got, changed)
	}
	if again, changed := AddPath(got, `c:\users\me\appdata\local\wintonicz\bin\`); changed || again != got {
		t.Fatalf("second AddPath changed the value: %q", again)
	}
	removed, changed := RemovePath(got, `C:\USERS\ME\APPDATA\LOCAL\WINTONICZ\BIN`)
	if !changed || removed != `C:\Windows;C:\Tools` {
		t.Fatalf("RemovePath = %q, %v", removed, changed)
	}
	if empty, _ := AddPath("", `C:\x`); empty != `C:\x` {
		t.Fatalf("AddPath to empty = %q", empty)
	}
}
