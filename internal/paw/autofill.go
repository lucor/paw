package paw

import "net/url"

type MatchTypeAutofill int

const (
	// DisabledAutofill disable the autofill feature
	DisabledAutofill MatchTypeAutofill = 0
	// ExactMatchAutofill match the exact URL along with the path (i.e. https://www.example.com/login but not https://www.example.com/login/1)
	ExactMatchAutofill MatchTypeAutofill = 2
	// DomainMatchAutofill match the domain only (i.e. https://example.com and https://example.com/login)
	DomainMatchAutofill MatchTypeAutofill = 4
	// SubdomainMatchAutofill match the subdomain only (i.e. https://www.example.com/login or https://www.example.com/auth but not https://dev.example.com/login)
	SubdomainMatchAutofill MatchTypeAutofill = 8
)

type Autofill struct {
	*url.URL   `json:"url,omitempty"`
	AllowHTTP  bool              `json:"allow_http,omitempty"`
	MatchType  MatchTypeAutofill `json:"match_type,omitempty"`
	TLDPlusOne string            `json:"tld_plus_one,omitempty"`
}
