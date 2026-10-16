package config

import (
	"strings"
	"testing"
)

func TestCompressSwitch(t *testing.T) {
	env := func(m map[string]string) func(string) (string, bool) {
		return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
	}
	c, err := Load(writeFile(t, "tollgate.yaml", minimal, 0o600), noEnv)
	if err != nil || c.Compress.Enabled {
		t.Fatalf("default: enabled=%v err=%v, want off", c != nil && c.Compress.Enabled, err)
	}
	p := writeFile(t, "tollgate.yaml", minimal+"compress:\n  enabled: true\n", 0o600)
	if c, err = Load(p, noEnv); err != nil || !c.Compress.Enabled {
		t.Fatalf("file: %v %v", c, err)
	}
	p = writeFile(t, "tollgate.yaml", minimal+"compress:\n  enabled: false\n", 0o600)
	if c, err = Load(p, env(map[string]string{"TOLLGATE_COMPRESS_ENABLED": "true"})); err != nil || !c.Compress.Enabled {
		t.Fatalf("env wins: %v %v", c, err)
	}
	_, err = Load(p, env(map[string]string{"TOLLGATE_COMPRESS_ENABLED": "vielleicht"}))
	if err == nil || !strings.Contains(err.Error(), "TOLLGATE_COMPRESS_ENABLED") || strings.Contains(err.Error(), "vielleicht") {
		t.Fatalf("invalid value: %v", err)
	}
}
