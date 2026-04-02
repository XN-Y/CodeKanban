//go:build linux

package terminal

import (
	"reflect"
	"testing"
)

func TestResizeSequenceLinuxSameSizeUsesRowNudge(t *testing.T) {
	got := resizeSequence("linux", 120, 40, 120, 40)
	want := []terminalSize{{cols: 120, rows: 41}, {cols: 120, rows: 40}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestResizeSequenceLinuxDifferentSizeUsesSingleResize(t *testing.T) {
	got := resizeSequence("linux", 120, 40, 120, 41)
	want := []terminalSize{{cols: 120, rows: 41}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestResizeSequenceNonLinuxSameSizeUsesSingleResize(t *testing.T) {
	got := resizeSequence("windows", 120, 40, 120, 40)
	want := []terminalSize{{cols: 120, rows: 40}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestResizeSequenceRejectsInvalidTarget(t *testing.T) {
	got := resizeSequence("linux", 120, 40, 0, 40)
	if len(got) != 0 {
		t.Fatalf("expected empty sequence, got %v", got)
	}
}
