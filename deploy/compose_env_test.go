package deploy

import (
	"testing"

	"go.yaml.in/yaml/v3"
)

// TestComposePassesStageSwitches checks that the redact and compress (#19)
// switches read by internal/config reach the tollgate container: redact off
// by default (until #90 changes it), compress on in the container.
func TestComposePassesStageSwitches(t *testing.T) {
	var doc struct {
		Services map[string]struct {
			Environment map[string]string `yaml:"environment"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal([]byte(readFile(t, "../docker-compose.yml")), &doc); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"TOLLGATE_REDACT_ENABLED":   "${TOLLGATE_REDACT_ENABLED:-false}",
		"TOLLGATE_COMPRESS_ENABLED": "${TOLLGATE_COMPRESS_ENABLED:-true}",
	} {
		if got := doc.Services["tollgate"].Environment[name]; got != want {
			t.Fatalf("tollgate environment %s = %q, want %q", name, got, want)
		}
	}
}
