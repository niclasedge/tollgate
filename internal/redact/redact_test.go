package redact

import (
	"reflect"
	"strings"
	"testing"
)

// fakeKey joins parts at runtime so that no key-shaped literal sits in the
// source (secret scanners).
func fakeKey(parts ...string) string { return strings.Join(parts, "") }

func TestBuiltinRuleSet(t *testing.T) {
	want := []string{
		"anthropic_key", "openai_key", "aws_access_key", "github_token", "slack_token", "stripe_live",
		"jwt", "pem_private", "email", "conn_string", "ipv4", "ipv6",
		"ps_secret_field", "azure_cred_guid", "cert_thumbprint", "client_secret",
		"auth_header", "bearer_token", "generic_assign",
	}
	var got []string
	for _, r := range builtinRules {
		got = append(got, r.name)
		if r.group >= r.re.NumSubexp()+1 {
			t.Errorf("%s: secret group %d out of range", r.name, r.group)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("rules = %q\nwant %q", got, want)
	}
}

func TestSwitches(t *testing.T) {
	inputs := map[string]string{
		"email":           "mail jane.roe@mailhost.de now",
		"api_keys":        "key " + fakeKey("AKIA", "IOSFODNN7EXAMPLE") + " now",
		"jwt":             "t eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ4In0.sigfake now",
		"connection_urls": "db postgres://app:s3cretpw@db:5432/x now",
		"ip_addresses":    "dns 8.8.8.8 now",
		"private_keys":    fakeKey("-----BEGIN PRIVATE ", "KEY-----\nabc\n-----END PRIVATE ", "KEY-----"),
	}
	setters := map[string]func(*Options, bool){
		"email":           func(o *Options, v bool) { o.Email = v },
		"api_keys":        func(o *Options, v bool) { o.APIKeys = v },
		"jwt":             func(o *Options, v bool) { o.JWT = v },
		"connection_urls": func(o *Options, v bool) { o.ConnectionURLs = v },
		"ip_addresses":    func(o *Options, v bool) { o.IPAddresses = v },
		"private_keys":    func(o *Options, v bool) { o.PrivateKeys = v },
	}
	for _, name := range []string{"email", "api_keys", "jwt", "connection_urls", "ip_addresses", "private_keys"} {
		t.Run(name, func(t *testing.T) {
			var on, off Options
			setters[name](&on, true)
			if got := New(on).Redact(inputs[name]); got == inputs[name] {
				t.Errorf("switch on: nothing redacted in %q", got)
			}
			if got := New(off).Redact(inputs[name]); got != inputs[name] {
				t.Errorf("all switches off: got %q", got)
			}
			all := corpusOptions()
			setters[name](&all, false)
			if fs := New(all).Find(inputs[name]); len(fs) != 0 {
				t.Errorf("switch off, others on: findings %+v", fs)
			}
		})
	}
}

func TestDefaultOptions(t *testing.T) {
	want := Options{Email: true, APIKeys: true, JWT: true, ConnectionURLs: true, IPAddresses: false, PrivateKeys: true}
	if got := DefaultOptions(); got != want {
		t.Fatalf("DefaultOptions() = %+v", got)
	}
	if got := New(DefaultOptions()).Redact("DNS 8.8.8.8 fragen"); got != "DNS 8.8.8.8 fragen" {
		t.Errorf("IPs are off by default, got %q", got)
	}
}

func TestResolution(t *testing.T) {
	anthropic := fakeKey("sk-", "ant-api03-", strings.Repeat("Ab1", 10))
	tests := []struct {
		name      string
		in        string
		want      string
		wantRules []string
	}{
		{"specific before generic", "api_key=" + anthropic, "api_key=<redacted:api_key>", []string{"anthropic_key"}},
		{"anthropic before openai on equal span", anthropic, "<redacted:api_key>", []string{"anthropic_key"}},
		{"jwt inside auth header", "Authorization: Bearer eyJa.eyJb.c", "Authorization: Bearer <redacted:jwt>", []string{"jwt"}},
		{"connection before email", "postgres://app:s3cretpw@db.prod.io/x", "postgres://app:<redacted:connection_password>@db.prod.io/x", []string{"conn_string"}},
		{"longer span wins among generics", "Authorization: Basic dXNlcjpwYXNzd29yZA== x", "Authorization: <redacted:api_key>", []string{"auth_header"}},
		{"equal span falls back to rule order", "X-API-Key: abc123def456", "X-API-Key: <redacted:api_key>", []string{"auth_header"}},
		{"prefix stays", "password = 'topsecret123'", "password = '<redacted:api_key>'", []string{"generic_assign"}},
		{"numbered placeholder is not a secret", "password=<redacted:api_key_1>", "password=<redacted:api_key_1>", nil},
		{"unnumbered label is text", "password=<redacted:email>", "password=<redacted:api_key>", []string{"generic_assign"}},
		{"two secrets in one line", "a@mailhost.de b@mailhost.de", "<redacted:email> <redacted:email>", []string{"email", "email"}},
		{"private address stays", "lo 127.0.0.1 pub 1.1.1.1", "lo 127.0.0.1 pub <redacted:ip>", []string{"ipv4"}},
		// Issue #83: the rest of a displaced generic finding is not left in plain text.
		{"residual of displaced generic", "Authorization: Bearer eyJa.eyJb.c weiterer-text", "Authorization: Bearer <redacted:jwt> <redacted:api_key>", []string{"jwt", "auth_header"}},
		{"residual between two specifics", "Authorization: a@mailhost.de mitte b@mailhost.de", "Authorization: <redacted:email> <redacted:api_key> <redacted:email>", []string{"email", "auth_header", "email"}},
		{"residual without letter or digit stays", "curl -H 'Authorization: Bearer eyJa.eyJb.c'", "curl -H 'Authorization: Bearer <redacted:jwt>'", []string{"jwt"}},
		{"key name of another rule stays", "Authorization: password=" + anthropic, "Authorization: password=<redacted:api_key>", []string{"anthropic_key"}},
	}
	r := New(corpusOptions())
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := r.Find(tt.in)
			var rules []string
			for i, f := range fs {
				rules = append(rules, f.Rule)
				if i > 0 && fs[i-1].End > f.Start {
					t.Errorf("findings overlap or are unsorted: %+v", fs)
				}
			}
			if !reflect.DeepEqual(rules, tt.wantRules) {
				t.Errorf("rules = %q, want %q", rules, tt.wantRules)
			}
			if got := r.Redact(tt.in); got != tt.want {
				t.Errorf("Redact = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReplace(t *testing.T) {
	text := "a SECRET b OTHER c"
	fs := []Finding{{Start: 2, End: 8, Type: "x"}, {Start: 11, End: 16, Type: "y"}}
	got := Replace(text, fs, func(f Finding) string { return "<" + f.Type + ">" })
	if got != "a <x> b <y> c" {
		t.Errorf("Replace = %q", got)
	}
	if got := Replace(text, nil, nil); got != text {
		t.Errorf("Replace without findings = %q", got)
	}
}

func TestIsPrivateIPv4(t *testing.T) {
	for s, want := range map[string]bool{
		"10.0.0.1": true, "127.0.0.1": true, "0.0.0.0": true, "192.168.1.1": true,
		"172.16.0.1": true, "172.31.255.254": true, "not.an.ip.x": true, "300.1.1.1": true,
		"8.8.8.8": false, "1.1.1.1": false, "172.15.0.1": false, "172.32.0.1": false,
	} {
		if got := isPrivateIPv4(s); got != want {
			t.Errorf("isPrivateIPv4(%q) = %v, want %v", s, got, want)
		}
	}
}

func TestIsPrivateIPv6(t *testing.T) {
	for s, want := range map[string]bool{
		"::": true, "::1": true, "fe80::1": true, "febf::ffff": true, "fc00::1": true,
		"fdff::ffff": true, "20:50:00": true,
		"2606:4700:4700::1111": false, "2001:db8::1": false, "fec0::1": false,
		"2001:db8:0:0:0:0:0:1": false,
	} {
		if got := isPrivateIPv6(s); got != want {
			t.Errorf("isPrivateIPv6(%q) = %v, want %v", s, got, want)
		}
	}
}

func TestIsFalsePositive(t *testing.T) {
	tests := []struct {
		matched, rule, before, after string
		want                         bool
	}{
		{"localhost", "anything", "", "", true},
		{"postgres://localhost/db", "conn_string", "", "", false},
		{"noreply@corp.io", "email", "", "", true},
		{"a@example.com", "email", "", "", true},
		{"a@mailhost.de", "email", "", "", false},
		{"1.2.3.4", "ipv4", "Version ", " released", true},
		{"1.2.3.4", "ipv4", "x v=(", " y", true},
		{"1.2.3.4", "ipv4", "x 'v'=(", " y", false},
		{"5.3.4.1", "ipv4", `PackageManagement "`, `" line`, true},
		{"5.3.4.1", "ipv4", `'`, `"`, false},
		{"8.8.8.8", "ipv4", "client=", " down", false},
		{"8.8.8.8", "ipv4", "versions ", "", false},
		{"8.8.8.8", "ipv4", "build, ", "", true},
	}
	for _, tt := range tests {
		if got := isFalsePositive(tt.matched, tt.rule, tt.before, tt.after); got != tt.want {
			t.Errorf("isFalsePositive(%q, %q, %q, %q) = %v, want %v", tt.matched, tt.rule, tt.before, tt.after, got, tt.want)
		}
	}
}

func TestContextAround(t *testing.T) {
	text := "äöü" + strings.Repeat("x", 20) + "MATCH" + strings.Repeat("y", 20)
	start := strings.Index(text, "MATCH")
	before, after := contextAround(text, start, start+5)
	if before != strings.Repeat("x", 16) || after != strings.Repeat("y", 16) {
		t.Errorf("context = %q / %q", before, after)
	}
	before, after = contextAround("ä1.2.3.4é", 2, 9)
	if before != "ä" || after != "é" {
		t.Errorf("short context = %q / %q", before, after)
	}
}

// TestDeterminism runs the whole corpus twice through fresh redactors and
// in parallel through a shared one: outputs and findings must be identical.
func TestDeterminism(t *testing.T) {
	cases := loadCorpus(t)
	var all strings.Builder
	for _, c := range cases {
		all.WriteString(c.input)
	}
	inputs := []string{all.String()}
	for _, c := range cases {
		inputs = append(inputs, c.input)
	}
	shared := New(corpusOptions())
	for _, in := range inputs {
		f1, f2 := New(corpusOptions()).Find(in), New(corpusOptions()).Find(in)
		if !reflect.DeepEqual(f1, f2) {
			t.Fatalf("findings differ for %q", in)
		}
		if a, b := shared.Redact(in), New(corpusOptions()).Redact(in); a != b {
			t.Fatalf("outputs differ for %q", in)
		}
	}
	t.Run("parallel", func(t *testing.T) {
		want := shared.Redact(inputs[0])
		for range 8 {
			t.Run("", func(t *testing.T) {
				t.Parallel()
				if got := shared.Redact(inputs[0]); got != want {
					t.Error("concurrent output differs")
				}
			})
		}
	})
}

// TestClientSecretBounds: a quoted client_secret value ends at the closing
// quote that matches the opening one; escaped quotes belong to the value.
// Unquoted values keep the \S{8,} span (#84).
func TestClientSecretBounds(t *testing.T) {
	r := New(DefaultOptions())
	tests := []struct{ in, want string }{
		{`client_secret: "S3cretPwd!Long",`, `client_secret: "<redacted:api_key>",`},
		{`client_secret='S3cretPwd!Long';`, `client_secret='<redacted:api_key>';`},
		{`{"client_secret":"S3cretPwd!Long"}`, `{"client_secret":"<redacted:api_key>"}`},
		{`{"client_secret":"abc\"defgh12","x":1}`, `{"client_secret":"<redacted:api_key>","x":1}`},
		{`{"client_secret":"abcdefg\\","x":1}`, `{"client_secret":"<redacted:api_key>","x":1}`},
		{`client_secret="abcde",x`, `client_secret="<redacted:api_key>",x`},
		{`client_secret="S3cretPwd!Long`, `client_secret="<redacted:api_key>`},
		{`ClientSecret=abcdef1234567890`, `ClientSecret=<redacted:api_key>`},
		{`client_secret=abcdefgh,`, `client_secret=<redacted:api_key>`},
	}
	for _, tt := range tests {
		if got := r.Redact(tt.in); got != tt.want {
			t.Errorf("Redact(%s) = %s, want %s", tt.in, got, tt.want)
		}
	}
	// A quoted empty value is no secret.
	if fs := r.Find(`client_secret="",abcdefgh`); len(fs) != 0 {
		t.Errorf("empty quoted value: %+v", fs)
	}
}
