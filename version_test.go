package msgbus

import (
	"fmt"
	"testing"
)

func TestFullVersion(t *testing.T) {
	version := FullVersion()

	expected := fmt.Sprintf("%s-%s@%s", Version, Build, GitCommit)

	if version != expected {
		t.Fatalf("invalid version returned: %s", version)
	}
}
