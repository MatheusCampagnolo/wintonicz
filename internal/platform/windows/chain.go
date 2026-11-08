//go:build windows

package windows

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/MatheusCampagnolo/wintonicz/internal/why"
)

// chainScript walks from one PID up through its parents. A parent created after its
// child is a reused PID, so the real parent has exited and the walk stops there.
const chainScript = `$id=%s; $out=@(); $note=""; $child=$null; $seen=@{};
for($i=0;$i -lt 12;$i++){
  $p=Get-CimInstance Win32_Process -Filter "ProcessId=$id" -ErrorAction SilentlyContinue
  if(-not $p){ if($child){ $note="The parent process has exited." }; break }
  if($child -and $p.CreationDate -gt $child.CreationDate){ $note="The parent process has exited (its PID was reused by an unrelated process)."; break }
  $out += [pscustomobject]@{ pid=[uint32]$p.ProcessId; parentPid=[uint32]$p.ParentProcessId; name=[string]$p.Name; path=[string]$p.ExecutablePath; commandLine=[string]$p.CommandLine; created=$p.CreationDate.ToString("o") }
  $child=$p
  if($p.ParentProcessId -eq 0 -or $seen.ContainsKey([int]$p.ParentProcessId)){ break }
  $seen[[int]$p.ProcessId]=1
  $id=$p.ParentProcessId
}
[pscustomobject]@{ chain=$out; note=$note } | ConvertTo-Json -Depth 4 -Compress`

func (Collector) ProcessChain(pid uint32) ([]why.Process, string, error) {
	// Only a decimal PID is ever placed in the script; nothing typed by a user reaches it.
	out, err := runPowerShell(fmt.Sprintf(chainScript, strconv.FormatUint(uint64(pid), 10)))
	if err != nil {
		return nil, "", fmt.Errorf("read process ancestry: %w", err)
	}
	var raw struct {
		Chain json.RawMessage `json:"chain"`
		Note  string          `json:"note"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, "", err
	}
	type row struct {
		PID         uint32 `json:"pid"`
		ParentPID   uint32 `json:"parentPid"`
		Name        string `json:"name"`
		Path        string `json:"path"`
		CommandLine string `json:"commandLine"`
		Created     string `json:"created"`
	}
	rows, err := jsonList[row](raw.Chain)
	if err != nil {
		return nil, "", err
	}
	chain := make([]why.Process, 0, len(rows))
	for _, r := range rows {
		created, _ := time.Parse(time.RFC3339Nano, r.Created)
		chain = append(chain, why.Process{PID: r.PID, ParentPID: r.ParentPID, Name: r.Name, Path: r.Path, CommandLine: r.CommandLine, Created: created})
	}
	return chain, raw.Note, nil
}
