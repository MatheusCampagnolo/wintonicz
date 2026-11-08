package redact

import (
	"strings"
	"testing"
)

func TestApplyMasksIdentifiersButKeepVersionsAndLoopback(t *testing.T) {
	r := New(All(), "matth", "MESSIAS")
	in := strings.Join([]string{
		`C:\Users\matth\AppData\Local\Temp\x.log`,
		`Host MESSIAS user matth`,
		`gateway 192.168.1.1 dns 8.8.8.8 loopback 127.0.0.1 any 0.0.0.0`,
		`build 10.0.26200.1`,
		`mac AA-BB-CC-DD-EE-FF and aa:bb:cc:dd:ee:ff`,
		`v6 fe80::1ff:fe23:4567:890a and 2606:4700:10::ac42:93f3 time 12:30:45`,
	}, "\n")
	out, c := r.Apply(in)
	for _, leak := range []string{"matth", "MESSIAS", "192.168.1.1", "8.8.8.8", "AA-BB-CC", "aa:bb", "fe80::", "2606:4700"} {
		if strings.Contains(out, leak) {
			t.Errorf("%q survived redaction:\n%s", leak, out)
		}
	}
	for _, keep := range []string{"127.0.0.1", "0.0.0.0", "10.0.26200.1", "12:30:45", `C:\Users\<user>\AppData`} {
		if !strings.Contains(out, keep) {
			t.Errorf("%q should have been kept:\n%s", keep, out)
		}
	}
	if c.IP != 4 || c.MAC != 2 || c.Host != 1 || c.Username < 2 {
		t.Errorf("counts = %+v", c)
	}
}

func TestShortUsernamesAreOnlyMaskedInProfilePaths(t *testing.T) {
	out, _ := New(All(), "me", "PC1").Apply(`C:\Users\me\Documents and the word me`)
	if !strings.Contains(out, `\Users\<user>\Documents`) || !strings.Contains(out, "the word me") {
		t.Errorf("unexpected output: %s", out)
	}
}

func TestParseOptions(t *testing.T) {
	if o, err := Parse("none"); err != nil || o != None() {
		t.Errorf("none = %+v, %v", o, err)
	}
	if o, err := Parse("ip,mac"); err != nil || !o.IP || !o.MAC || o.Username {
		t.Errorf("ip,mac = %+v, %v", o, err)
	}
	if _, err := Parse("passwords"); err == nil {
		t.Error("an unknown option must be rejected")
	}
}

func TestFourPartVersionsThatLookLikeAddressesAreMaskedOnPurpose(t *testing.T) {
	// 7.4.1.2 is a valid IPv4 literal; masking it is the safe side of the trade-off.
	if out, _ := New(All(), "x", "y").Apply("agent 7.4.1.2"); out != "agent <ipv4>" {
		t.Errorf("got %q", out)
	}
}
