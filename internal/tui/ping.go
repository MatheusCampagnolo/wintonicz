package tui

import (
	"fmt"
	"net"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/MatheusCampagnolo/wintonicz/internal/network"
)

// Stopper is implemented by screens that keep working in the background (a live
// ping). The shell calls Stop when the user leaves them.
type Stopper interface{ Stop() }

type resolveMsg struct {
	gen int
	ip  net.IP
	err error
}
type pingTick struct{ gen int }
type pingResultMsg struct {
	gen int
	res network.PingResult
	err error
}

// pingScreen sends one echo per second and draws a live latency graph.
type pingScreen struct {
	pinger  network.Pinger
	gen     int
	editing bool
	draft   string
	pos     int
	started bool
	running bool
	target  string
	ip      net.IP
	results []network.PingResult
	err     error
}

func newPingScreen(p network.Pinger) *pingScreen { return &pingScreen{pinger: p, draft: "1.1.1.1"} }

func (p *pingScreen) Init() tea.Cmd {
	if !p.started {
		p.started, p.editing = true, true
		p.pos = len([]rune(p.draft))
	}
	return nil
}

func (p *pingScreen) Stop()         { p.gen++; p.running = false }
func (p *pingScreen) Editing() bool { return p.editing }
func (p *pingScreen) Dirty() bool   { return false }
func (p *pingScreen) Details(int) string {
	return "Sends one ICMP echo per second (IPv4). Uses the Windows ICMP helper, so no administrator rights are needed. Leaving this screen stops it."
}
func (p *pingScreen) Title() string {
	if p.running {
		return "PING · " + p.target + " · live"
	}
	return "PING"
}

func (p *pingScreen) Hints() []hint {
	switch {
	case p.editing:
		return []hint{{"enter", "start"}, {"esc", "cancel"}}
	case p.running:
		return []hint{{"s", "stop"}, {"c", "change host"}}
	}
	return []hint{{"enter", "start again"}, {"c", "change host"}}
}

func (p *pingScreen) begin(host string) tea.Cmd {
	p.gen++
	gen := p.gen
	p.target, p.results, p.err, p.editing = host, nil, nil, false
	return func() tea.Msg {
		ip, err := network.ResolveIPv4(host)
		return resolveMsg{gen: gen, ip: ip, err: err}
	}
}

func (p *pingScreen) sendOne() tea.Cmd {
	gen, ip, pinger := p.gen, p.ip, p.pinger
	return func() tea.Msg {
		res, err := pinger.Ping(ip, 2*time.Second)
		return pingResultMsg{gen: gen, res: res, err: err}
	}
}

func (p *pingScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case resolveMsg:
		if msg.gen != p.gen {
			return p, nil
		}
		if msg.err != nil {
			p.err = msg.err
			return p, nil
		}
		p.ip, p.running = msg.ip, true
		return p, p.sendOne()
	case pingResultMsg:
		if msg.gen != p.gen {
			return p, nil
		}
		if msg.err != nil {
			p.err, p.running = msg.err, false
			return p, nil
		}
		msg.res.Seq = len(p.results) + 1
		p.results = append(p.results, msg.res)
		gen := p.gen
		return p, tea.Tick(time.Second, func(time.Time) tea.Msg { return pingTick{gen: gen} })
	case pingTick:
		if msg.gen == p.gen && p.running {
			return p, p.sendOne()
		}
	case tea.KeyPressMsg:
		if p.editing {
			switch msg.String() {
			case "enter":
				if host := strings.TrimSpace(p.draft); host != "" {
					return p, p.begin(host)
				}
			case "esc":
				p.editing = false
			default:
				p.draft, p.pos = editTextDraft(p.draft, p.pos, msg, 253)
			}
			return p, nil
		}
		switch msg.String() {
		case "s":
			p.Stop()
		case "c":
			p.Stop()
			p.editing, p.pos = true, len([]rune(p.draft))
		case "enter":
			return p, p.begin(strings.TrimSpace(p.draft))
		}
	}
	return p, nil
}

const sparkChars = "▁▂▃▄▅▆▇█"

func (p *pingScreen) View(w, h int) string {
	lines := []string{"Host or IPv4 address"}
	if p.editing {
		lines = append(lines, "> "+renderDraft(p.draft, p.pos), "", muted("Press Enter to start."))
		return strings.Join(lines, "\n")
	}
	lines = append(lines, "> "+p.draft)
	if p.err != nil {
		return strings.Join(append(lines, "", danger("✗ "+p.err.Error()), "", muted("Press c to change the host.")), "\n")
	}
	if len(p.results) == 0 {
		return strings.Join(append(lines, "", muted("Resolving and sending the first echo…")), "\n")
	}
	width := max(10, w-2)
	shown := p.results
	if len(shown) > width {
		shown = shown[len(shown)-width:]
	}
	peak := int64(10)
	for _, r := range shown {
		if r.OK && r.RTTMillis > peak {
			peak = r.RTTMillis
		}
	}
	var graph strings.Builder
	for _, r := range shown {
		if !r.OK {
			graph.WriteString(danger("×"))
			continue
		}
		idx := int(float64(r.RTTMillis) / float64(peak) * float64(len([]rune(sparkChars))-1))
		graph.WriteString(accent(string([]rune(sparkChars)[idx])))
	}
	last := p.results[len(p.results)-1]
	lastText := okText(fmt.Sprintf("%d ms", last.RTTMillis))
	if !last.OK {
		lastText = danger(last.Status)
	}
	st := network.Summarize(p.results)
	lines = append(lines, "", graph.String(), muted(fmt.Sprintf("scale 0–%d ms · × = no reply", peak)), "",
		fmt.Sprintf("Last      %s  (%s)", lastText, last.Address),
		fmt.Sprintf("Sent      %d   Received %d   Loss %.0f%%", st.Sent, st.Received, st.LossPercent()),
		fmt.Sprintf("Min/avg/max  %d / %d / %d ms", st.MinMS, st.AvgMS, st.MaxMS))
	return strings.Join(lines, "\n")
}
