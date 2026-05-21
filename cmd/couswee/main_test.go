package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunVersionPrintsAndExits(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"--version"}, &out); err != nil {
		t.Fatalf("run --version: %v", err)
	}
	got := out.String()
	for _, want := range []string{"couswee dev", "commit none", "built unknown"} {
		if !strings.Contains(got, want) {
			t.Fatalf("version output = %q, missing %q", got, want)
		}
	}
}

func TestRunShortVersionPrintsAndExits(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"-v"}, &out); err != nil {
		t.Fatalf("run -v: %v", err)
	}
	if !strings.Contains(out.String(), "couswee dev") {
		t.Fatalf("version output = %q", out.String())
	}
}
