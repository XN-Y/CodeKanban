package system

import (
	"errors"
	"reflect"
	"testing"
)

func TestBuildCustomEditorCommandPartsUsesCurrentPlaceholder(t *testing.T) {
	parts, err := buildCustomEditorCommandParts(`code --reuse-window ${path}`, "/tmp/worktree")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := []string{"code", "--reuse-window", "/tmp/worktree"}
	if !reflect.DeepEqual(parts, expected) {
		t.Fatalf("expected %v, got %v", expected, parts)
	}
}

func TestBuildCustomEditorCommandPartsAppendsPathWhenPlaceholderMissing(t *testing.T) {
	parts, err := buildCustomEditorCommandParts(`code --reuse-window`, "/tmp/worktree")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := []string{"code", "--reuse-window", "/tmp/worktree"}
	if !reflect.DeepEqual(parts, expected) {
		t.Fatalf("expected %v, got %v", expected, parts)
	}
}

func TestBuildCustomEditorCommandPartsRejectsEmptyCommand(t *testing.T) {
	_, err := buildCustomEditorCommandParts("   ", "/tmp/worktree")
	if !errors.Is(err, ErrCustomEditorCommand) {
		t.Fatalf("expected ErrCustomEditorCommand, got %v", err)
	}
}
