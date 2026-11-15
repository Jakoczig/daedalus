package desktop

import "testing"

func TestWaypointClipboardReportsActualResult(t *testing.T) {
	c := &Controller{}
	if c.CopyWaypoint("Fixture") == nil {
		t.Fatal("unbound clipboard success")
	}
	calls := 0
	c.BindClipboard(func(s string) bool { calls++; return false })
	if c.CopyWaypoint("Fixture") == nil || calls != 1 {
		t.Fatal("failed write claimed success")
	}
	c.BindClipboard(func(s string) bool { calls++; return s == "Fixture" })
	if e := c.CopyWaypoint("Fixture"); e != nil {
		t.Fatal(e)
	}
	if c.CopyWaypoint("") == nil {
		t.Fatal("empty accepted")
	}
}
