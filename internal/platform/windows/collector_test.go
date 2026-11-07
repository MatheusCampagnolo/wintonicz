//go:build windows

package windows

import (
	"encoding/binary"
	"testing"
)

func TestParseIPv4ListenerRow(t *testing.T) {
	buffer := make([]byte, 24)
	binary.LittleEndian.PutUint32(buffer[0:4], tcpStateListen)
	binary.LittleEndian.PutUint32(buffer[4:8], 0x0100007f)
	binary.LittleEndian.PutUint32(buffer[8:12], 0x0000901f)
	binary.LittleEndian.PutUint32(buffer[20:24], 1234)

	rows := parseIPv4Listeners(buffer, 1)
	if len(rows) != 1 {
		t.Fatalf("listener count = %d, want 1", len(rows))
	}
	if rows[0].Address != "127.0.0.1" || rows[0].Port != 8080 || rows[0].PID != 1234 {
		t.Fatalf("parsed listener = %+v", rows[0])
	}
}

func TestParseIPv6ListenerRow(t *testing.T) {
	buffer := make([]byte, 56)
	buffer[15] = 1
	binary.LittleEndian.PutUint32(buffer[20:24], 0x0000bb01)
	binary.LittleEndian.PutUint32(buffer[48:52], tcpStateListen)
	binary.LittleEndian.PutUint32(buffer[52:56], 4321)

	rows := parseIPv6Listeners(buffer, 1)
	if len(rows) != 1 {
		t.Fatalf("listener count = %d, want 1", len(rows))
	}
	if rows[0].Address != "::1" || rows[0].Port != 443 || rows[0].PID != 4321 {
		t.Fatalf("parsed listener = %+v", rows[0])
	}
}
