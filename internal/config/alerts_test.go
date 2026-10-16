package config

import (
	"strings"
	"testing"
)

func TestAlertsConfig(t *testing.T) {
	env := func(m map[string]string) func(string) (string, bool) {
		return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
	}
	c, err := Load(writeFile(t, "tollgate.yaml", minimal, 0o600), noEnv)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Alerts{FiveHourThreshold: 90}); c.Alerts != want {
		t.Fatalf("defaults = %+v, want %+v", c.Alerts, want)
	}
	p := writeFile(t, "tollgate.yaml", minimal+"alerts:\n  spool_dir: /alerts\n  auto_switch: true\n  five_hour_threshold: 80\n", 0o600)
	if c, err = Load(p, noEnv); err != nil || c.Alerts != (Alerts{SpoolDir: "/alerts", AutoSwitch: true, FiveHourThreshold: 80}) {
		t.Fatalf("file: %+v %v", c.Alerts, err)
	}
	c, err = Load(p, env(map[string]string{
		"TOLLGATE_ALERTS_SPOOL_DIR": "/spool", "TOLLGATE_ALERTS_AUTO_SWITCH": "false", "TOLLGATE_ALERTS_FIVE_HOUR_THRESHOLD": "95",
	}))
	if err != nil || c.Alerts != (Alerts{SpoolDir: "/spool", FiveHourThreshold: 95}) {
		t.Fatalf("env wins: %+v %v", c.Alerts, err)
	}
	for k, v := range map[string]string{"TOLLGATE_ALERTS_AUTO_SWITCH": "jein", "TOLLGATE_ALERTS_FIVE_HOUR_THRESHOLD": "neunzig"} {
		if _, err := Load(p, env(map[string]string{k: v})); err == nil || !strings.Contains(err.Error(), k) {
			t.Errorf("%s=%s: err = %v", k, v, err)
		}
	}
	for _, th := range []string{"-1", "101"} {
		c, err := Load(p, env(map[string]string{"TOLLGATE_ALERTS_FIVE_HOUR_THRESHOLD": th}))
		if err != nil {
			t.Fatal(err)
		}
		if err := c.Validate(true); err == nil || !strings.Contains(err.Error(), "five_hour_threshold") {
			t.Errorf("threshold %s: Validate = %v", th, err)
		}
	}
}
