package services

import (
	"net/url"
	"strings"
)

const maxArvisClaimURLBytes = 8192

// arvisClaimRedirectEvidence is a bounded, offline-only interpretation of
// user-supplied URL query values. It neither follows redirects nor authenticates
// a target origin, website, transaction or contract.
type arvisClaimRedirectEvidence struct {
	Hosts                       []string
	CrossOriginCandidateCount   int
	NonHTTPSchemeCandidateCount int
	ParsedRedirectCount         int
}

func analyzeArvisClaimURLQuery(source *url.URL) ([]string, []string, []string, []string, arvisClaimRedirectEvidence) {
	secretTerms := []string{}
	signingTerms := []string{}
	redirectTerms := []string{}
	promotionTerms := []string{}
	redirects := arvisClaimRedirectEvidence{Hosts: []string{}}
	if source == nil {
		return secretTerms, signingTerms, redirectTerms, promotionTerms, redirects
	}
	query := source.Query()
	for key, values := range query {
		lowerKey := strings.ToLower(strings.TrimSpace(key))
		secretTerms = appendUniqueStrings(secretTerms, matchedClaimTerms(lowerKey, claimSecretIndicators)...)
		signingTerms = appendUniqueStrings(signingTerms, matchedClaimTerms(lowerKey, claimSigningIndicators)...)
		redirectTerms = appendUniqueStrings(redirectTerms, matchedClaimTerms(lowerKey, claimRedirectIndicators)...)
		promotionTerms = appendUniqueStrings(promotionTerms, matchedClaimTerms(lowerKey, claimPromotionIndicators)...)
		for _, value := range values {
			// url.URL.Query() performs one decoding pass. We do not recursively
			// decode, evaluate, fetch, resolve DNS or execute the supplied value.
			decoded := strings.ToLower(strings.TrimSpace(value))
			secretTerms = appendUniqueStrings(secretTerms, matchedClaimTerms(decoded, claimSecretIndicators)...)
			signingTerms = appendUniqueStrings(signingTerms, matchedClaimTerms(decoded, claimSigningIndicators)...)
			redirectTerms = appendUniqueStrings(redirectTerms, matchedClaimTerms(decoded, claimRedirectIndicators)...)
			promotionTerms = appendUniqueStrings(promotionTerms, matchedClaimTerms(decoded, claimPromotionIndicators)...)
			if !arvisClaimRedirectKey(lowerKey) {
				continue
			}
			target, err := url.Parse(strings.TrimSpace(value))
			if err != nil {
				continue
			}
			scheme := strings.ToLower(strings.TrimSpace(target.Scheme))
			if scheme != "" && scheme != "http" && scheme != "https" {
				redirects.NonHTTPSchemeCandidateCount++
				continue
			}
			host := strings.ToLower(strings.TrimSpace(target.Hostname()))
			if host == "" {
				continue
			}
			redirects.ParsedRedirectCount++
			redirects.Hosts = appendUniqueStrings(redirects.Hosts, host)
			if !arvisSameURLOrigin(source, target) {
				redirects.CrossOriginCandidateCount++
			}
		}
	}
	return secretTerms, signingTerms, redirectTerms, promotionTerms, redirects
}

var claimSecretIndicators = []string{
	"seed", "seedphrase", "mnemonic", "privatekey", "private_key", "secretkey",
	"secret_key", "recoveryphrase", "recovery_phrase",
}

var claimSigningIndicators = []string{
	"sign", "signature", "transaction", "approve", "authorize", "connectwallet",
	"connect_wallet", "walletconnect", "wallet_connect",
}

var claimRedirectIndicators = []string{
	"redirect", "redirect_uri", "returnurl", "return_url", "callback", "continue", "next",
}

var claimPromotionIndicators = []string{
	"claim", "airdrop", "reward", "bonus", "mint", "whitelist", "presale", "verify",
}

func arvisClaimRedirectKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "redirect", "redirect_uri", "redirect_url", "returnurl", "return_url",
		"callback", "callback_url", "continue", "next", "url", "target", "destination":
		return true
	default:
		return false
	}
}

func arvisSameURLOrigin(source, target *url.URL) bool {
	if source == nil || target == nil {
		return false
	}
	if !strings.EqualFold(source.Scheme, target.Scheme) ||
		!strings.EqualFold(source.Hostname(), target.Hostname()) {
		return false
	}
	// An explicit alternate port is a different web origin, even when the
	// hostname and scheme match. Equivalent standard/default ports match.
	originPort := func(value *url.URL) string {
		if port := value.Port(); port != "" {
			return port
		}
		if strings.EqualFold(value.Scheme, "https") {
			return "443"
		}
		if strings.EqualFold(value.Scheme, "http") {
			return "80"
		}
		return ""
	}
	return originPort(source) == originPort(target)
}
