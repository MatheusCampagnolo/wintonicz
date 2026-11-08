package runner

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
	"unicode/utf16"
)

func utf16le(s string) []byte {
	var b bytes.Buffer
	for _, u := range utf16.Encode([]rune(s)) {
		b.WriteByte(byte(u))
		b.WriteByte(byte(u >> 8))
	}
	return b.Bytes()
}

func TestWideProgressOutputIsDecodedAndSplitOnCarriageReturns(t *testing.T) {
	raw := utf16le("Verification 10% complete.\rVerification 100% complete.\r\n\r\nDone.\r\n")
	reader, wide := decodingReader(bytes.NewReader(raw))
	if !wide {
		t.Fatal("UTF-16 output was not detected")
	}
	scanner := bufio.NewScanner(reader)
	scanner.Split(splitLines)
	var got []string
	for scanner.Scan() {
		got = append(got, scanner.Text())
	}
	want := "Verification 10% complete.|Verification 100% complete.|Done."
	if strings.Join(got, "|") != want {
		t.Fatalf("lines = %q, want %q", got, want)
	}
}

func TestPlainOutputIsNotTreatedAsWide(t *testing.T) {
	if _, wide := decodingReader(strings.NewReader("hello\n")); wide {
		t.Fatal("ASCII output was misdetected as UTF-16")
	}
}
