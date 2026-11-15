package linkkeys

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"
	"unicode"
)

var ErrNoBrowserEndpoint = errors.New("domain has no public LinkKeys browser endpoint")
var ErrInvalidBrowserEndpoint = errors.New("domain has an invalid LinkKeys browser endpoint")

// ParseBrowserLogin accepts a username and an ASCII DNS domain.
// The username is only a form hint. It does not prove identity.
func ParseBrowserLogin(login string) (username, domain string, err error) {
	login = strings.TrimSpace(login)
	username, domain, ok := strings.Cut(login, "@")
	domain = strings.ToLower(domain)
	if !ok || username == "" || strings.Contains(domain, "@") ||
		strings.ContainsAny(username, "/\\?#") || strings.ContainsFunc(login, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) || !validLoginDomain(domain) {
		return "", "", errors.New("enter a login in the form name@domain.com")
	}
	return username, domain, nil
}

func validLoginDomain(domain string) bool {
	if len(domain) > 253 || !strings.Contains(domain, ".") || net.ParseIP(domain) != nil {
		return false
	}
	for _, label := range strings.Split(domain, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}

type TXTResolver interface {
	LookupTXT(context.Context, string) ([]string, error)
}

// DiscoverBrowserEndpoint requires a published HTTPS endpoint for this flow.
// An absent record must not send the user to an unrelated website.
func DiscoverBrowserEndpoint(ctx context.Context, resolver TXTResolver, domain string) (string, error) {
	records, err := resolver.LookupTXT(ctx, "_linkkeys_apis."+domain)
	if err != nil {
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
			return "", ErrNoBrowserEndpoint
		}
		return "", err
	}
	for _, record := range records {
		fields := parseLinkkeysTXT(record)
		if fields["v"] != "lk1" || fields["https"] == "" {
			continue
		}
		endpoint := fields["https"]
		// The record can contain a host with an optional path prefix.
		if !strings.Contains(endpoint, "://") {
			endpoint = "https://" + endpoint
		}
		u, err := url.Parse(endpoint)
		if err != nil || u.Scheme != "https" || !validLoginDomain(strings.ToLower(u.Hostname())) || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return "", ErrInvalidBrowserEndpoint
		}
		return strings.TrimRight(u.String(), "/"), nil
	}
	return "", ErrNoBrowserEndpoint
}
