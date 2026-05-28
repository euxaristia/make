package main

import "testing"

func TestDagOrder(t *testing.T) {
	d := NewDag()
	d.AddPrereq("all", "a")
	d.AddPrereq("all", "b")
	d.AddPrereq("a", "c")

	gotNodes := d.Order("all")
	got := make([]string, 0, len(gotNodes))
	for _, n := range gotNodes {
		got = append(got, n.Target)
	}

	want := []string{"c", "a", "b", "all"}
	if len(got) != len(want) {
		t.Fatalf("Order() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Order() = %v, want %v", got, want)
		}
	}
}

func TestDagDetectCycle(t *testing.T) {
	d := NewDag()
	d.AddPrereq("a", "b")
	d.AddPrereq("b", "c")
	d.AddPrereq("c", "a")

	cycle := d.DetectCycle()
	if len(cycle) == 0 {
		t.Fatal("DetectCycle() returned no cycle")
	}

	seen := make(map[string]bool)
	for _, name := range cycle {
		seen[name] = true
	}
	for _, name := range []string{"a", "b", "c"} {
		if !seen[name] {
			t.Fatalf("DetectCycle() = %v, want it to include %q", cycle, name)
		}
	}
}
