package monitor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/instabox"
	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/store"
)

// Time-window scheduling lets a machine run only inside user-defined daily windows
// (e.g. 06:00-08:00, 18:00-19:00). Windows are wall-clock minutes in a fixed UTC
// offset (default +07:00) and never cross midnight — users split overnight spans
// into two windows. Enforcement runs inside the regular poll cycle:
//   - inside a window:  stopped/suspended machines are started/woken
//   - outside windows:  running/starting machines are stopped
// The schedule always wins over manual stops inside a window (same philosophy as
// keepalive), and enabling it disables keepalive — both modes would fight otherwise.

type scheduleWindow struct {
	StartMin int    `json:"-"`
	EndMin   int    `json:"-"`
	Start    string `json:"start"`
	End      string `json:"end"`
}

func parseHHMM(s string) (int, error) {
	var h, m int
	if len(s) != 5 || s[2] != ':' {
		return 0, errors.New("not HH:MM")
	}
	if _, err := fmt.Sscanf(s, "%d:%d", &h, &m); err != nil {
		return 0, err
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, errors.New("out of range")
	}
	return h*60 + m, nil
}

// ParseScheduleWindows validates a JSON array of {"start":"HH:MM","end":"HH:MM"} and
// returns windows sorted by start minute. Overlapping, empty or inverted windows are
// rejected; a window ending at 00:00 means end-of-day (1440).
func ParseScheduleWindows(raw string) ([]scheduleWindow, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("windows are required")
	}
	var entries []scheduleWindow
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return nil, fmt.Errorf("invalid windows JSON: %w", err)
	}
	if len(entries) == 0 {
		return nil, errors.New("at least one window is required")
	}
	if len(entries) > 12 {
		return nil, errors.New("at most 12 windows per day")
	}
	for i := range entries {
		startMin, err := parseHHMM(entries[i].Start)
		if err != nil {
			return nil, fmt.Errorf("window %d start: %v", i+1, err)
		}
		endMin, err := parseHHMM(entries[i].End)
		if err != nil {
			return nil, fmt.Errorf("window %d end: %v", i+1, err)
		}
		if endMin == 0 {
			endMin = 1440 // 00:00 as an end means end-of-day
		}
		if startMin >= endMin {
			return nil, fmt.Errorf("window %d (%s-%s): end must be after start — split overnight spans at midnight", i+1, entries[i].Start, entries[i].End)
		}
		if endMin-startMin < 5 {
			return nil, fmt.Errorf("window %d (%s-%s): windows must be at least 5 minutes", i+1, entries[i].Start, entries[i].End)
		}
		entries[i].StartMin = startMin
		entries[i].EndMin = endMin
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].StartMin < entries[j].StartMin })
	for i := 1; i < len(entries); i++ {
		if entries[i].StartMin < entries[i-1].EndMin {
			return nil, fmt.Errorf("windows %s-%s and %s-%s overlap", entries[i-1].Start, entries[i-1].End, entries[i].Start, entries[i].End)
		}
	}
	return entries, nil
}

// ValidateScheduleWindowsText parses the free-text UI form "06:00-08:00, 18:00-19:00"
// into normalized JSON. Empty input is an error.
func ValidateScheduleWindowsText(text string) (normalizedJSON string, err error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", errors.New("at least one window is required")
	}
	// Accept already-normalized JSON (e.g. MCP callers reusing a stored value).
	if strings.HasPrefix(text, "[") {
		wins, err := ParseScheduleWindows(text)
		if err != nil {
			return "", err
		}
		return mustJSON(wins), nil
	}
	parts := strings.FieldsFunc(text, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\t'
	})
	entries := make([]scheduleWindow, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		bounds := strings.Split(part, "-")
		if len(bounds) != 2 {
			return "", fmt.Errorf("window %q: use HH:MM-HH:MM", part)
		}
		entries = append(entries, scheduleWindow{
			Start: strings.TrimSpace(bounds[0]),
			End:   strings.TrimSpace(bounds[1]),
		})
	}
	wins, err := ParseScheduleWindows(mustJSON(entries))
	if err != nil {
		return "", err
	}
	return mustJSON(wins), nil
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

// WindowsText renders normalized JSON back into the compact UI form.
func WindowsText(raw string) string {
	wins, err := ParseScheduleWindows(raw)
	if err != nil {
		return raw
	}
	parts := make([]string, len(wins))
	for i, w := range wins {
		end := w.End
		if w.EndMin == 1440 {
			end = "00:00"
		}
		parts[i] = w.Start + "-" + end
	}
	return strings.Join(parts, ", ")
}

// InWindow reports whether minute-of-day `min` falls inside any window.
func InWindow(wins []scheduleWindow, min int) bool {
	for _, w := range wins {
		if min >= w.StartMin && min < w.EndMin {
			return true
		}
	}
	return false
}

// NextTransition describes the next boundary change for the UI countdown.
func NextTransition(wins []scheduleWindow, min int) (kind string, inMinutes int) {
	if InWindow(wins, min) {
		best := -1
		for _, w := range wins {
			if min >= w.StartMin && min < w.EndMin {
				d := w.EndMin - min
				if best == -1 || d < best {
					best = d
				}
			}
		}
		return "stop", best
	}
	best := -1
	for _, w := range wins {
		var d int
		if min < w.StartMin {
			d = w.StartMin - min
		} else {
			d = 1440 - min + w.StartMin
		}
		if best == -1 || d < best {
			best = d
		}
	}
	return "start", best
}

// minuteOfDayInOffset returns minutes since midnight in the schedule's fixed offset.
func minuteOfDayInOffset(now time.Time, offsetMin int) int {
	shifted := now.UTC().Add(time.Duration(offsetMin) * time.Minute)
	return shifted.Hour()*60 + shifted.Minute()
}

// enforceSchedule evaluates one machine's schedule against its current state and
// performs the required start/wake/stop. Called from the poll cycle after telemetry.
func (p *Poller) enforceSchedule(ctx context.Context, accountID, apiKey string, m instabox.Machine, rec store.MachineRecord) {
	if !rec.ScheduleEnabled || isTransitionalState(m.State) {
		return
	}
	wins, err := ParseScheduleWindows(rec.ScheduleWindows)
	if err != nil || len(wins) == 0 {
		if rec.ScheduleLastError == "" {
			_ = p.store.UpdateMachineScheduleState(ctx, accountID, m.ID, time.Now().Unix(), "error", "invalid or empty schedule windows")
		}
		return
	}
	offset := rec.ScheduleTZOffsetMin
	if offset == 0 {
		offset = 420
	}
	nowMin := minuteOfDayInOffset(p.upstreamNow(), offset)
	active := InWindow(wins, nowMin)

	record := func(action, errMsg string) {
		// Skip rewriting the same action at the same minute to reduce churn.
		_ = p.store.UpdateMachineScheduleState(ctx, accountID, m.ID, time.Now().Unix(), action, errMsg)
	}

	isCooldown := isCooldownActive(m, p.upstreamNow())

	switch {
	case active && m.State == "stopped":
		if isCooldown {
			record("cooldown_wait", "")
			return
		}
		res, err := p.client.PerformAction(ctx, apiKey, m.ID, instabox.ActionStart)
		if err != nil {
			record("error", err.Error())
			log.Printf("schedule start %s/%s: %v", accountID, m.ID, err)
			return
		}
		if res != nil && res.Machine != nil {
			_ = p.syncSingleMachine(ctx, accountID, *res.Machine)
		}
		if res != nil && res.Operation != nil && res.Operation.State != "succeeded" && res.Operation.State != "failed" {
			p.trackOperation(accountID, apiKey, res.Operation.ID)
		}
		record("window_start", "")

	case active && m.State == "suspended":
		if isCooldown {
			record("cooldown_wait", "")
			return
		}
		action := instabox.ActionWake
		if m.AutoSuspendedAt == "" {
			action = instabox.ActionResume
		}
		res, err := p.client.PerformAction(ctx, apiKey, m.ID, action)
		if err != nil {
			record("error", err.Error())
			log.Printf("schedule wake %s/%s: %v", accountID, m.ID, err)
			return
		}
		if res != nil && res.Machine != nil {
			_ = p.syncSingleMachine(ctx, accountID, *res.Machine)
		}
		if res != nil && res.Operation != nil && res.Operation.State != "succeeded" && res.Operation.State != "failed" {
			p.trackOperation(accountID, apiKey, res.Operation.ID)
		}
		record("window_wake", "")

	case !active && (m.State == "running" || m.State == "starting"):
		res, err := p.client.PerformAction(ctx, apiKey, m.ID, instabox.ActionStop)
		if err != nil {
			record("error", err.Error())
			log.Printf("schedule stop %s/%s: %v", accountID, m.ID, err)
			return
		}
		if res != nil && res.Machine != nil {
			_ = p.syncSingleMachine(ctx, accountID, *res.Machine)
		}
		if res != nil && res.Operation != nil && res.Operation.State != "succeeded" && res.Operation.State != "failed" {
			p.trackOperation(accountID, apiKey, res.Operation.ID)
		}
		record("window_stop", "")

	default:
		// In the correct state for the current window: refresh the status row at
		// most once every 15 minutes so the UI shows a live "in_window"/"off_window".
		if time.Now().Unix()-rec.ScheduleLastAt > 900 || (rec.ScheduleLastAction != "in_window" && rec.ScheduleLastAction != "off_window") {
			if active {
				record("in_window", "")
			} else {
				record("off_window", "")
			}
		}
	}
}

// SetMachineSchedule validates and persists a schedule, toggling keepalive off when
// enabling. Returns the refreshed machine record.
func (p *Poller) SetMachineSchedule(ctx context.Context, accountID, machineID string, enabled bool, windowsText string, tzOffsetMin int) (*store.MachineRecord, error) {
	if err := p.CheckMachineExists(ctx, accountID, machineID); err != nil {
		return nil, err
	}
	normalized := ""
	clearWindows := false
	if enabled {
		if tzOffsetMin == 0 {
			tzOffsetMin = 420
		}
		if tzOffsetMin < -720 || tzOffsetMin > 840 {
			return nil, errors.New("timezone offset must be between -12h and +14h")
		}
		var err error
		normalized, err = ValidateScheduleWindowsText(windowsText)
		if err != nil {
			return nil, err
		}
	} else if strings.TrimSpace(windowsText) == "[]" {
		// Disabling normally preserves the configured windows for re-enable, but an
		// explicit empty list means "clear my schedule" (the editor's wipe-and-disable).
		clearWindows = true
	}
	if err := p.store.SetMachineSchedule(ctx, accountID, machineID, enabled, normalized, tzOffsetMin, clearWindows); err != nil {
		return nil, err
	}
	if enabled {
		key := accountID + ":" + machineID
		p.cancelKeepaliveCycle(ctx, key, 2*time.Second)
		_ = p.store.SetMachineKeepalivePendingRestart(ctx, accountID, machineID, false)
		p.deleteKeepaliveLeaseRunStart(key)
	}
	return p.store.GetMachine(ctx, accountID, machineID)
}
