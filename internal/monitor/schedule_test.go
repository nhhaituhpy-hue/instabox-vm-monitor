package monitor

import (
	"strings"
	"testing"
	"time"
)

func TestValidateScheduleWindowsText(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"single window", "06:00-08:00", ""},
		{"multiple windows", "06:00-08:00, 11:00-12:00, 18:00-19:00", ""},
		{"semicolon separated", "06:00-08:00; 18:00-19:00", ""},
		{"end of day midnight", "20:00-00:00", ""},
		{"empty input", "", "at least one window"},
		{"inverted window", "18:00-06:00", "end must be after start"},
		{"zero length", "08:00-08:00", "end must be after start"},
		{"too short", "08:00-08:03", "at least 5 minutes"},
		{"bad format", "8am-10am", "not HH:MM"},
		{"missing dash", "08:00", "use HH:MM-HH:MM"},
		{"bad hour", "25:00-26:00", "start"},
		{"overlap", "06:00-09:00, 08:00-10:00", "overlap"},
		{"touching is fine", "06:00-09:00, 09:00-10:00", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ValidateScheduleWindowsText(tc.input)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got == "" || !strings.HasPrefix(got, "[") {
					t.Fatalf("expected normalized JSON, got %q", got)
				}
			} else {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil (result %q)", tc.wantErr, got)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error %q should contain %q", err.Error(), tc.wantErr)
				}
			}
		})
	}
}

func TestValidateScheduleWindowsTextNormalizesAndSorts(t *testing.T) {
	got, err := ValidateScheduleWindowsText("18:00-19:00, 06:00-08:00")
	if err != nil {
		t.Fatal(err)
	}
	wins, err := ParseScheduleWindows(got)
	if err != nil {
		t.Fatal(err)
	}
	if len(wins) != 2 || wins[0].StartMin != 360 || wins[1].StartMin != 1080 {
		t.Fatalf("expected sorted windows 06:00 then 18:00, got %+v", wins)
	}
}

func TestValidateScheduleWindowsTextAcceptsJSON(t *testing.T) {
	first, err := ValidateScheduleWindowsText("06:00-08:00")
	if err != nil {
		t.Fatal(err)
	}
	// Round-trip: passing normalized JSON back in must succeed (MCP re-use path).
	second, err := ValidateScheduleWindowsText(first)
	if err != nil {
		t.Fatalf("JSON round-trip failed: %v", err)
	}
	if first != second {
		t.Fatalf("round-trip mismatch: %q vs %q", first, second)
	}
}

func TestInWindowAndNextTransition(t *testing.T) {
	wins, err := ParseScheduleWindows(mustValidate(t, "06:00-08:00, 11:00-12:00, 18:00-19:00"))
	if err != nil {
		t.Fatal(err)
	}
	at := func(h, m int) int { return h*60 + m }

	if !InWindow(wins, at(6, 30)) {
		t.Error("06:30 should be inside the first window")
	}
	if InWindow(wins, at(8, 0)) {
		t.Error("08:00 exactly should be outside (end exclusive)")
	}
	if InWindow(wins, at(23, 59)) {
		t.Error("23:59 should be outside all windows")
	}

	kind, mins := NextTransition(wins, at(7, 30))
	if kind != "stop" || mins != 30 {
		t.Errorf("at 07:30 expected stop in 30m, got %s in %dm", kind, mins)
	}
	kind, mins = NextTransition(wins, at(9, 0))
	if kind != "start" || mins != 120 {
		t.Errorf("at 09:00 expected start in 120m, got %s in %dm", kind, mins)
	}
	kind, mins = NextTransition(wins, at(20, 0))
	if kind != "start" || mins != 600 {
		t.Errorf("at 20:00 expected next-day start in 600m, got %s in %dm", kind, mins)
	}
}

func TestMinuteOfDayInOffset(t *testing.T) {
	// 2026-11-26 23:30 UTC is 06:30 next day at UTC+7.
	ts := time.Date(2026, 11, 26, 23, 30, 0, 0, time.UTC)
	got := minuteOfDayInOffset(ts, 420)
	if got != 6*60+30 {
		t.Fatalf("expected 390 (06:30), got %d", got)
	}
}

func TestWindowsTextRoundTrip(t *testing.T) {
	norm := mustValidate(t, "06:00-08:00, 18:00-19:00")
	text := WindowsText(norm)
	if text != "06:00-08:00, 18:00-19:00" {
		t.Fatalf("unexpected round-trip text: %q", text)
	}
}

func mustValidate(t *testing.T, input string) string {
	t.Helper()
	got, err := ValidateScheduleWindowsText(input)
	if err != nil {
		t.Fatalf("validate %q: %v", input, err)
	}
	return got
}
