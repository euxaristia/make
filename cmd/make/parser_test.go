package main

import (
	"reflect"
	"testing"
)

func TestParseMakefile(t *testing.T) {
	d := NewDag()
	content := `
CC = cc
OBJ = main.o util.o

prog: $(OBJ)
	$(CC) -o $@ $^

.PHONY: clean
clean:
	rm -f prog $(OBJ)
`

	if errMsg := Parse(content, d); errMsg != "" {
		t.Fatalf("Parse() returned %q", errMsg)
	}

	if d.DefaultTarget != "prog" {
		t.Fatalf("DefaultTarget = %q, want %q", d.DefaultTarget, "prog")
	}
	if d.Variables["CC"] != "cc" {
		t.Fatalf("CC = %q, want %q", d.Variables["CC"], "cc")
	}

	prog := d.Nodes["prog"]
	if prog == nil {
		t.Fatal("prog node was not created")
	}
	if !reflect.DeepEqual(prog.Prereqs, []string{"main.o", "util.o"}) {
		t.Fatalf("prog prereqs = %v, want [main.o util.o]", prog.Prereqs)
	}
	if !reflect.DeepEqual(prog.Recipes, []string{"$(CC) -o $@ $^"}) {
		t.Fatalf("prog recipes = %v, want compiler recipe", prog.Recipes)
	}

	clean := d.Nodes["clean"]
	if clean == nil {
		t.Fatal("clean node was not created")
	}
	if !clean.IsPhony {
		t.Fatal("clean should be marked phony")
	}
}

func TestStripComment(t *testing.T) {
	tests := []struct {
		line string
		want string
	}{
		{line: "A = value # comment", want: "A = value"},
		{line: `A = value \# literal`, want: `A = value \# literal`},
		{line: "A = $(shell printf '#') # comment", want: "A = $(shell printf '#')"},
	}

	for _, tt := range tests {
		if got := stripComment(tt.line); got != tt.want {
			t.Fatalf("stripComment(%q) = %q, want %q", tt.line, got, tt.want)
		}
	}
}
