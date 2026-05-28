package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExpanderSimpleVariables(t *testing.T) {
	exp := &Expander{Vars: map[string]string{
		"A": "hello",
		"B": "$(A) world",
	}}

	if got, want := exp.Simple("$(B)"), "hello world"; got != want {
		t.Fatalf("Simple() = %q, want %q", got, want)
	}
}

func TestExpanderSubstitutionReference(t *testing.T) {
	exp := &Expander{Vars: map[string]string{
		"OBJ": "main.o util.o README",
	}}

	if got, want := exp.Simple("$(OBJ:.o=.c)"), "main.c util.c README"; got != want {
		t.Fatalf("Simple() = %q, want %q", got, want)
	}
}

func TestExpanderAutomaticVariables(t *testing.T) {
	exp := &Expander{Vars: map[string]string{}}
	auto := &AutoVars{
		Target:  "program",
		Prereqs: []string{"main.o", "util.o", "main.o"},
	}

	got := exp.WithAuto("$@|$<|$^|$+", auto)
	want := "program|main.o|main.o util.o|main.o util.o main.o"
	if got != want {
		t.Fatalf("WithAuto() = %q, want %q", got, want)
	}
}

func TestExpanderNewerPrereqs(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	oldPrereq := filepath.Join(dir, "old")
	newPrereq := filepath.Join(dir, "new")

	for _, path := range []string{target, oldPrereq, newPrereq} {
		if err := os.WriteFile(path, []byte(path), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	base := time.Now().Add(-1 * time.Hour)
	if err := os.Chtimes(oldPrereq, base, base); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(target, base.Add(10*time.Minute), base.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newPrereq, base.Add(20*time.Minute), base.Add(20*time.Minute)); err != nil {
		t.Fatal(err)
	}

	exp := &Expander{Vars: map[string]string{}}
	auto := &AutoVars{Target: target, Prereqs: []string{oldPrereq, newPrereq}}

	if got, want := exp.WithAuto("$?", auto), newPrereq; got != want {
		t.Fatalf("WithAuto($?) = %q, want %q", got, want)
	}
}
