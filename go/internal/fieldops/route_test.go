package fieldops

import (
	"strings"
	"testing"
)

func TestCSVBoundsAndArrivalAuthority(t *testing.T) {
	p, e := ParseCSV([]byte("\ufeffSystem Name,Neutron,Refuel\n\"Fixture, α\",true,false\nBeta,false,true\n"))
	if e != nil || len(p) != 2 || p[0].Name != "Fixture, α" || !p[0].Neutron {
		t.Fatal(p, e)
	}
	for _, s := range []string{"name\nX\n", "system\nhttps://evil.test\n", "system\nA\nA\n", "system,x,y,z\nA,NaN,1,2\n", "system\n=cmd\n"} {
		if _, e := ParseCSV([]byte(s)); e == nil {
			t.Fatal("invalid accepted", s)
		}
	}
	max := "system\n"
	for i := 0; i < 2048; i++ {
		max += strings.Repeat("x", i%30+1) + string(rune(0x1000+i)) + "\n"
	}
	if _, e := ParseCSV([]byte(max)); e != nil {
		t.Fatal(e)
	}
	if _, e := ParseCSV([]byte(max + "overflow\n")); e == nil {
		t.Fatal("unbounded")
	}
	plan := []Waypoint{{Name: "Alpha", Address: "9007199254740993"}, {Name: "Beta"}, {Name: "Gamma"}}
	if n, s := Advance(plan, 0, "FSDTarget", "Alpha", "9007199254740993"); n != 0 || s != "unchanged" {
		t.Fatal(n, s)
	}
	if n, s := Advance(plan, 0, "FSDJump", "Alpha", "9007199254740992"); n != 0 || s != "off_route" {
		t.Fatal(n, s)
	}
	if n, s := Advance(plan, 0, "Location", "Alpha", "9007199254740993"); n != 1 || s != "address_verified" {
		t.Fatal(n, s)
	}
	if n, s := Advance(plan, 1, "FSDJump", "Gamma", ""); n != 1 || s != "off_route" {
		t.Fatal("skipped ahead", n, s)
	}
	if n, _ := Advance(plan, 1, "FSDJump", "Beta", ""); n != 2 {
		t.Fatal(n)
	}
}
