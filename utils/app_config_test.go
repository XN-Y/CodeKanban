package utils

import (
	"reflect"
	"testing"
)

func TestNormalizeWebSessionQuickInputConfig(t *testing.T) {
	input := WebSessionQuickInputConfig{
		Pinned: []string{"  Alpha  ", "", "Beta", "Alpha"},
		Recent: []string{" One ", "Two", "One", "", "Three", "Four", "Five", "Six", "Seven"},
	}

	got := NormalizeWebSessionQuickInputConfig(input)
	want := WebSessionQuickInputConfig{
		Pinned: []string{"Alpha", "Beta"},
		Recent: []string{"One", "Two", "Three", "Four", "Five", "Six"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeWebSessionQuickInputConfig() = %#v, want %#v", got, want)
	}
}

func TestNormalizeWebSessionQuickInputConfigEmpty(t *testing.T) {
	got := NormalizeWebSessionQuickInputConfig(WebSessionQuickInputConfig{})
	want := WebSessionQuickInputConfig{
		Pinned: []string{},
		Recent: []string{},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeWebSessionQuickInputConfig() = %#v, want %#v", got, want)
	}
}
