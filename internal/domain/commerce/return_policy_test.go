package commerce

import (
	"testing"
	"time"
)

func TestClaimDeadlines(t *testing.T) {
	d := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		kind   string
		offset time.Duration
		want   bool
	}{{"damage", 48 * time.Hour, true}, {"damage", 48*time.Hour + time.Nanosecond, false}, {"wrong_item", -time.Second, false}, {"change_of_mind", 72 * time.Hour, true}, {"change_of_mind", 72*time.Hour + time.Second, false}} {
		if got := ClaimWithinWindow(c.kind, d, d.Add(c.offset)); got != c.want {
			t.Errorf("%s %s=%v", c.kind, c.offset, got)
		}
	}
	if ClaimWithinWindow("damage", time.Time{}, d) {
		t.Fatal("missing delivery accepted")
	}
}
func TestCreditCalendarExpiryAndRestoration(t *testing.T) {
	for _, c := range []struct{ issued, want string }{{"2026-08-31", "2027-02-28"}, {"2027-08-31", "2028-02-29"}, {"2026-11-16", "2027-05-16"}} {
		issued, _ := time.Parse("2006-01-02", c.issued)
		if got := CreditExpiry(issued).Format("2006-01-02"); got != c.want {
			t.Errorf("%s expires %s", c.issued, got)
		}
	}
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	if !RestoredCreditExpiry(now.Add(-time.Hour), now).Equal(now.Add(30 * 24 * time.Hour)) {
		t.Fatal("expired credit lost grace")
	}
	later := now.Add(60 * 24 * time.Hour)
	if !RestoredCreditExpiry(later, now).Equal(later) {
		t.Fatal("valid credit shortened")
	}
}

func TestBusinessDeadlineSkipsClosedHoursAndSunday(t *testing.T) {
	zone := time.FixedZone("IST", 19800)
	for _, c := range []struct {
		start, want string
		hours       int
	}{{"2026-11-20 18:00", "2026-11-21 10:00", 2}, {"2026-11-21 18:00", "2026-11-23 10:00", 2}, {"2026-11-22 12:00", "2026-11-23 10:00", 1}} {
		start, _ := time.ParseInLocation("2006-01-02 15:04", c.start, zone)
		if got := BusinessDeadline(start, c.hours).Format("2006-01-02 15:04"); got != c.want {
			t.Errorf("%s: got %s want %s", c.start, got, c.want)
		}
	}
}
