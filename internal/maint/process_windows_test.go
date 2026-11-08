//go:build windows

package maint

import (
	"context"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/MatheusCampagnolo/wintonicz/internal/platform/windows"
)

func TestEndProcessEndsOnlyTheNamedProcessAndRefusesProtectedOnes(t *testing.T) {
	cmd := exec.Command("ping.exe", "-n", "60", "127.0.0.1")
	if err := cmd.Start(); err != nil {
		t.Skip("cannot start ping:", err)
	}
	defer cmd.Process.Kill()

	lister := windows.Collector{}.ListProcesses
	plan, err := endProcessPlan(map[string]string{"pid": strconv.Itoa(cmd.Process.Pid)}, lister)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Preview.ConfirmWord != strconv.Itoa(cmd.Process.Pid) {
		t.Errorf("confirm word = %q, want the PID", plan.Preview.ConfirmWord)
	}
	if err := plan.Do(context.Background(), func(string) {}); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the process is still running after End process")
	}

	all, _ := lister()
	for _, e := range all {
		if e.Name == "lsass.exe" {
			if _, err := endProcessPlan(map[string]string{"pid": strconv.Itoa(int(e.PID))}, lister); err == nil {
				t.Fatal("lsass.exe must be protected")
			}
		}
	}
}
