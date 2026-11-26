package monitor

import (
	"fmt"
	"testing"
	"time"

	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/instabox"
	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/store"
)

// maintTestOffset returns a tz offset that places the current UTC time at
// exactly local 10:00, so test windows can use fixed daytime HH:MM values.
func maintTestOffset() int {
	utcNow := time.Now().UTC()
	utcMin := utcNow.Hour()*60 + utcNow.Minute()
	// want: (utcMin + offset) mod 1440 == 600 (10:00)
	offset := (600 - utcMin) % 1440
	if offset < 0 {
		offset += 1440
	}
	return offset
}

func maintRecord(offset int, startHHMM, endHHMM string) store.MachineRecord {
	return store.MachineRecord{
		AccountID: "acct", MachineID: "m1",
		ScheduleEnabled: true,
		ScheduleWindows: fmt.Sprintf(`[{"start":"%s","end":"%s"}]`, startHHMM, endHHMM),
		ScheduleTZOffsetMin: offset,
	}
}

func TestMaintTestOffsetPlacesNowAtTenAM(t *testing.T) {
	offset := maintTestOffset()
	nowMin := minuteOfDayInOffset(time.Now().UTC(), offset)
	if nowMin < 595 || nowMin > 605 {
		t.Fatalf("expected local minute near 600, got %d", nowMin)
	}
}

func TestNeedsScheduleMaintenanceShortWindowSkipped(t *testing.T) {
	p := &Poller{}
	// current time is 10:00; window 09:55-10:15 is 20 minutes < 29m threshold
	rec := maintRecord(maintTestOffset(), "09:55", "10:15")
	m := instabox.Machine{ID: "m1", State: "running", MaxRunMinutes: 480}
	if p.needsScheduleMaintenance(m, rec) {
		t.Fatal("20-minute window must not need maintenance (idle pause cannot trigger)")
	}
}

func TestNeedsScheduleMaintenance29MinuteBoundary(t *testing.T) {
	p := &Poller{}
	rec := maintRecord(maintTestOffset(), "09:55", "10:24") // exactly 29 minutes
	rs := time.Now().UTC().Add(-3 * time.Minute).Format(time.RFC3339)
	m := instabox.Machine{ID: "m1", State: "running", MaxRunMinutes: 480, RunStartedAt: rs}
	if !p.needsScheduleMaintenance(m, rec) {
		t.Fatal("29-minute window must need maintenance (boundary is inclusive)")
	}
}

func TestNeedsScheduleMaintenanceLongWindowFirstPing(t *testing.T) {
	p := &Poller{}
	rec := maintRecord(maintTestOffset(), "09:55", "12:55") // 3h window
	rs := time.Now().UTC().Add(-3 * time.Minute).Format(time.RFC3339)
	m := instabox.Machine{ID: "m1", State: "running", MaxRunMinutes: 480, RunStartedAt: rs}
	if !p.needsScheduleMaintenance(m, rec) {
		t.Fatal("fresh run inside a 3h window should trigger the first activity ping")
	}
	// After a ping 5 minutes ago, no new ping is due.
	p.keepaliveLastPing.Store("acct:m1", time.Now().Unix()-5*60)
	p.keepaliveLeaseRunStart.Store("acct:m1", rs)
	if p.needsScheduleMaintenance(m, rec) {
		t.Fatal("ping 5 minutes ago — next ping not due yet")
	}
	// After 21 minutes the ping is due again.
	p.keepaliveLastPing.Store("acct:m1", time.Now().Unix()-21*60)
	if !p.needsScheduleMaintenance(m, rec) {
		t.Fatal("ping 21 minutes ago — a new ping is due")
	}
}

func TestNeedsScheduleMaintenancePowerCycleWindowClamping(t *testing.T) {
	p := &Poller{}
	rs := time.Now().UTC().Add(-8 * time.Hour).Format(time.RFC3339) // way over the 7h cap
	m := instabox.Machine{ID: "m1", State: "running", MaxRunMinutes: 480, RunStartedAt: rs}

	// Window closes in 10 minutes (now is 10:00): power-cycle must NOT fire.
	rec := maintRecord(maintTestOffset(), "05:00", "10:10")
	if p.needsScheduleMaintenance(m, rec) {
		t.Fatal("window closes in 10m — power-cycle would boot past the close, must be skipped")
	}

	// Window closes in 30 minutes: power-cycle is due.
	rec = maintRecord(maintTestOffset(), "05:00", "10:30")
	if !p.needsScheduleMaintenance(m, rec) {
		t.Fatal("runtime past cap with 30m left in window — power-cycle is due")
	}
}

func TestNeedsScheduleMaintenanceOutsideWindow(t *testing.T) {
	p := &Poller{}
	rec := maintRecord(maintTestOffset(), "06:00", "09:55") // window ended at 09:55, now 10:00
	m := instabox.Machine{ID: "m1", State: "running", MaxRunMinutes: 480}
	if p.needsScheduleMaintenance(m, rec) {
		t.Fatal("outside the window there is nothing to maintain — enforcement owns the machine")
	}
}

func TestNeedsScheduleMaintenanceNotRunning(t *testing.T) {
	p := &Poller{}
	rec := maintRecord(maintTestOffset(), "09:55", "12:55")
	for _, st := range []string{"stopped", "suspended", "stopping", "starting"} {
		m := instabox.Machine{ID: "m1", State: st, MaxRunMinutes: 480}
		if p.needsScheduleMaintenance(m, rec) {
			t.Fatalf("state %q must not trigger maintenance", st)
		}
	}
}
