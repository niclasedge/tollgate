// Package accounts manages encrypted Claude credentials and manual switching.
package accounts

// Account contains public metadata, never the token file.
type Account struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Email       string         `json:"email"`
	Active      bool           `json:"active"`
	Status      string         `json:"status"`
	ExpiresAt   string         `json:"expires_at"`
	LastRefresh string         `json:"last_refresh"`
	UpdatedAt   string         `json:"updated_at"`
	Usage       *UsageSnapshot `json:"usage,omitempty"`
}
type ListResponse struct {
	Accounts []Account `json:"accounts"`
}
type Login struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	ExpiresAt string `json:"expires_at"`
}
type TestResult struct {
	OK      bool   `json:"ok"`
	Status  int    `json:"status"`
	Model   string `json:"model"`
	Message string `json:"message"`
}
type Activation struct {
	Account     Account    `json:"account"`
	Test        TestResult `json:"test"`
	CacheNotice string     `json:"cache_notice"`
}

// UsageWindow is a percentage from the subscription usage endpoint.
type UsageWindow struct {
	ObservedAt  string   `json:"observed_at,omitempty"`
	Utilization *float64 `json:"utilization" tstype:"number | null,required"`
	Reset       string   `json:"reset"`
}
type UsageSnapshot struct {
	FiveHour   UsageWindow `json:"five_hour"`
	SevenDay   UsageWindow `json:"seven_day"`
	ObservedAt string      `json:"observed_at"`
	Error      string      `json:"error"`
	License    *License    `json:"license,omitempty"`
}

// Token states of a License.
const (
	TokenValid   = "valid"
	TokenExpired = "expired" // access token past its expiry; CPA renews it
	TokenRelogin = "relogin" // rejected before expiry; a new login is needed
	TokenUnknown = "unknown"
)

// License is the last known subscription state. Plan fields are fixed
// identifiers from the OAuth profile, never names, UUIDs or tokens.
type License struct {
	Token              string `json:"token"`
	CheckedAt          string `json:"checked_at"`
	OrganizationType   string `json:"organization_type"`
	RateLimitTier      string `json:"rate_limit_tier"`
	SubscriptionStatus string `json:"subscription_status"`
	ProfileObservedAt  string `json:"profile_observed_at"`
}

// SwitchStatus is available without the manager lock while a switch drains.
type SwitchStatus struct {
	Waiting         bool `json:"waiting"`
	Switching       bool `json:"switching"`
	RunningRequests int  `json:"running_requests"`
}
