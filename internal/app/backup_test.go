package app

import (
	"io"
	"strings"
	"testing"
)

type spaces struct{}

func (spaces) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = ' '
	}
	return len(p), nil
}

func TestBackupInputValidationIsSharedByStreams(t *testing.T) {
	for _, input := range []string{`{"version":1,"unknown":true}`, `{"version":1} {}`, `{"version":1`, `null`} {
		if _, err := DecodeMappingBackup(strings.NewReader(input)); err == nil {
			t.Fatalf("accepted %q", input)
		}
	}
	// Even otherwise valid JSON followed by oversized whitespace must be rejected.
	input := io.MultiReader(strings.NewReader(`{"version":1}`), io.LimitReader(spaces{}, 64<<20))
	if _, err := DecodeMappingBackup(input); err == nil || !strings.Contains(err.Error(), "64 MiB") {
		t.Fatalf("input limit: %v", err)
	}
}
