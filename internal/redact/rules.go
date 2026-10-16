package redact

import "regexp"

// Options switches the built-in detector groups (ADR-0005, ADR-0008
// `builtin.*`).
type Options struct {
	Email          bool
	APIKeys        bool
	JWT            bool
	ConnectionURLs bool
	IPAddresses    bool
	PrivateKeys    bool
}

// DefaultOptions returns the default switches: everything on except IP
// addresses.
func DefaultOptions() Options {
	return Options{
		Email:          true,
		APIKeys:        true,
		JWT:            true,
		ConnectionURLs: true,
		IPAddresses:    false,
		PrivateKeys:    true,
	}
}

// Placeholder types of the built-in rules (ADR-0005, ADR-0006).
const (
	TypeEmail              = "email"
	TypeAPIKey             = "api_key"
	TypeJWT                = "jwt"
	TypeConnectionPassword = "connection_password"
	TypeIP                 = "ip"
	TypePrivateKey         = "private_key"
)

// Priorities for overlap resolution. Values of 100 and above are reserved
// for word lists and custom rules (issue #15).
const (
	prioConnection = 30 // beats email: user:pass@host.tld reads as an e-mail
	prioSpecific   = 20 // value-shaped rules: vendor keys, JWT, PEM, e-mail, IPs
	prioGeneric    = 10 // key-name and header heuristics
)

type switchFunc func(Options) bool

type rule struct {
	name     string
	re       *regexp.Regexp
	group    int // capture group holding the secret; 0 = whole match
	typ      string
	priority int
	enabled  switchFunc
	find     func(text string) [][]int // optional matcher (word list); nil uses re
	// need lists literals of which every match contains at least one
	// (prefilter, #84). nil means the rule always runs. For rules with
	// fold the literals are lower-case and compared case-insensitively.
	need []string
	fold bool
	// trim narrows the secret group [start, end) of a match; nil keeps it.
	trim func(text string, start, end int) (int, int)
}

func onEmail(o Options) bool       { return o.Email }
func onAPIKeys(o Options) bool     { return o.APIKeys }
func onJWT(o Options) bool         { return o.JWT }
func onConnection(o Options) bool  { return o.ConnectionURLs }
func onIP(o Options) bool          { return o.IPAddresses }
func onPrivateKeys(o Options) bool { return o.PrivateKeys }

// builtinRules lists the ported rules. The regex texts are taken verbatim
// from hook-redactor v0.8.1 src/patterns.rs. Where the Rust rule replaced a
// prefix via template ("$1=...", "$1$2 ..."), the value part is the secret
// group instead; for auth_header, azure_cred_guid, cert_thumbprint and
// client_secret a capture group was added around the value, which does not
// change what the regex matches. conn_string uses the new pattern from
// ADR-0005. The order is the final tie-breaker in overlap resolution.
//
// need (prefilter, #84) lists literals of which every match of the regex
// contains at least one; fold marks (?i) rules, whose literals are matched
// case-insensitively. TestPrefilterKeepsCorpusFindings and FuzzPrefilter
// check that the prefilter never loses a finding.
var builtinRules = []rule{
	{name: "anthropic_key", re: regexp.MustCompile(`\bsk-ant-(?:api03-)?[A-Za-z0-9_\-]{20,}`), typ: TypeAPIKey, priority: prioSpecific, enabled: onAPIKeys, need: []string{"sk-ant-"}},
	{name: "openai_key", re: regexp.MustCompile(`\bsk-(?:proj-)?[A-Za-z0-9_\-]{20,}`), typ: TypeAPIKey, priority: prioSpecific, enabled: onAPIKeys, need: []string{"sk-"}},
	{name: "aws_access_key", re: regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`), typ: TypeAPIKey, priority: prioSpecific, enabled: onAPIKeys, need: []string{"AKIA"}},
	{name: "github_token", re: regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{30,}\b`), typ: TypeAPIKey, priority: prioSpecific, enabled: onAPIKeys, need: []string{"ghp_", "gho_", "ghu_", "ghs_", "ghr_"}},
	{name: "slack_token", re: regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}\b`), typ: TypeAPIKey, priority: prioSpecific, enabled: onAPIKeys, need: []string{"xox"}},
	{name: "stripe_live", re: regexp.MustCompile(`\b(?:sk|pk|rk)_live_[A-Za-z0-9]{20,}\b`), typ: TypeAPIKey, priority: prioSpecific, enabled: onAPIKeys, need: []string{"_live_"}},
	{name: "jwt", re: regexp.MustCompile(`\beyJ[A-Za-z0-9_\-]+\.eyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+\b`), typ: TypeJWT, priority: prioSpecific, enabled: onJWT, need: []string{"eyJ"}},
	{name: "pem_private", re: regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----`), typ: TypePrivateKey, priority: prioSpecific, enabled: onPrivateKeys, need: []string{"-----BEGIN "}},
	{name: "email", re: regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`), typ: TypeEmail, priority: prioSpecific, enabled: onEmail, need: []string{"@"}},
	{name: "conn_string", re: regexp.MustCompile(`(?i)\b(postgres(?:ql)?|mysql|mongodb(?:\+srv)?|redis|rediss|amqps?)://[^\s:/@]+:([^\s@/]+)@`), group: 2, typ: TypeConnectionPassword, priority: prioConnection, enabled: onConnection, need: []string{"://"}, fold: true},
	{name: "ipv4", re: regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`), typ: TypeIP, priority: prioSpecific, enabled: onIP, need: []string{"."}},
	{name: "ipv6", re: regexp.MustCompile(`\b(?:[0-9A-Fa-f]{1,4}:){2,7}(?::(?:[0-9A-Fa-f]{1,4}(?::[0-9A-Fa-f]{1,4})*)?|[0-9A-Fa-f]{1,4})\b`), typ: TypeIP, priority: prioSpecific, enabled: onIP, need: []string{":"}},
	{name: "ps_secret_field", re: regexp.MustCompile(`(?i)\b(ClientSecret|RefreshToken|AccessToken|ApiKey|AppPassword|PSCred|VaultPassword|SafeMode(?:Admin)?Password|ServicePassword)\s*=\s*["']?([^"'\s,;]{8,})["']?`), group: 2, typ: TypeAPIKey, priority: prioGeneric, enabled: onAPIKeys, need: []string{"clientsecret", "refreshtoken", "accesstoken", "apikey", "apppassword", "pscred", "vaultpassword", "safemode", "servicepassword"}, fold: true},
	{name: "azure_cred_guid", re: regexp.MustCompile(`(?i)(TenantId|ClientId|AppId|ApplicationId|ObjectId|SubscriptionId|DirectoryId|PrincipalId)["']?\s*[:=]\s*["']?([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})["']?`), group: 2, typ: TypeAPIKey, priority: prioGeneric, enabled: onAPIKeys, need: []string{"tenantid", "clientid", "appid", "applicationid", "objectid", "subscriptionid", "directoryid", "principalid"}, fold: true},
	{name: "cert_thumbprint", re: regexp.MustCompile(`(?i)(CertificateThumbprint|Thumbprint|FingerPrint)["']?\s*[:=]\s*["']?([0-9A-Fa-f]{40})["']?`), group: 2, typ: TypeAPIKey, priority: prioGeneric, enabled: onAPIKeys, need: []string{"thumbprint", "fingerprint"}, fold: true},
	{name: "client_secret", re: regexp.MustCompile(`(?i)(ClientSecret|client_secret)["']?\s*[:=]\s*["']?(\S{8,})["']?`), group: 2, typ: TypeAPIKey, priority: prioGeneric, enabled: onAPIKeys, need: []string{"clientsecret", "client_secret"}, fold: true, trim: trimQuoted},
	{name: "auth_header", re: regexp.MustCompile(`(?i)\b(authorization|proxy-authorization|x-api-key|x-auth-token|x-access-token)\s*([:=])\s*(\S.*)`), group: 3, typ: TypeAPIKey, priority: prioGeneric, enabled: onAPIKeys, need: []string{"authorization", "x-api-key", "x-auth-token", "x-access-token"}, fold: true},
	{name: "bearer_token", re: regexp.MustCompile(`(?i)\b(Bearer|Basic|Digest)\s+([A-Za-z0-9._\-+/=]{16,})`), group: 2, typ: TypeAPIKey, priority: prioGeneric, enabled: onAPIKeys, need: []string{"bearer", "basic", "digest"}, fold: true},
	{name: "generic_assign", re: regexp.MustCompile(`(?i)\b(password|passwd|pwd|secret|api[_-]?key|access[_-]?token|auth[_-]?token|bearer)\s*([:=])\s*['"]?([^\s'"]{6,})['"]?`), group: 3, typ: TypeAPIKey, priority: prioGeneric, enabled: onAPIKeys, need: []string{"password", "passwd", "pwd", "secret", "api", "access", "auth", "bearer"}, fold: true},
}

// trimQuoted ends a value that follows an opening quote at the first
// unescaped matching quote, so the closing quote and what follows it (comma,
// brace, next JSON field) stay readable (#84). A backslash escapes the next
// byte. Unquoted values and values without a closing quote keep their span.
func trimQuoted(text string, start, end int) (int, int) {
	if start == 0 {
		return start, end
	}
	q := text[start-1]
	if q != '"' && q != '\'' {
		return start, end
	}
	for i := start; i < end; i++ {
		switch text[i] {
		case '\\':
			i++
		case q:
			return start, i
		}
	}
	return start, end
}
