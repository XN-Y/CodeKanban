package terminal

import (
	"bytes"
	"testing"

	"github.com/tuzig/vt10x"
)

func TestVT10XWithXtermStyleRepliesSecondaryDAAsXterm(t *testing.T) {
	var reply bytes.Buffer

	term := vt10x.New(
		vt10x.WithXtermStyle(),
		vt10x.WithWriter(&reply),
	)

	if _, err := term.Write([]byte("\x1b[>c")); err != nil {
		t.Fatalf("write DA2 probe: %v", err)
	}

	if got := reply.String(); got != "\x1b[>0;276;0c" {
		t.Fatalf("secondary DA reply = %q, want %q", got, "\x1b[>0;276;0c")
	}
}
