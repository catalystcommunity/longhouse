package linkkeys

import (
	"context"
	"errors"
	"net"
	"testing"
)

type browserTXT struct {
	records []string
	err     error
	name    string
}

func (r *browserTXT) LookupTXT(_ context.Context, name string) ([]string, error) {
	r.name = name
	return r.records, r.err
}

func TestParseBrowserLogin(t *testing.T) {
	u, d, err := ParseBrowserLogin("  Foo+bar@EXAMPLE.com  ")
	if err != nil || u != "Foo+bar" || d != "example.com" {
		t.Fatalf("%q %q %v", u, d, err)
	}
	for _, value := range []string{"", "example.com", "@example.com", "a@@example.com", "a@https://example.com", "a@localhost", "a@127.0.0.1", "a@-example.com", "a@example..com", "a@example.com/path", "a@example.com:443", "a b@example.com", "a\n@example.com"} {
		if _, _, err := ParseBrowserLogin(value); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
}

func TestDiscoverBrowserEndpoint(t *testing.T) {
	for _, tc := range []struct {
		name, record, want string
		err                error
	}{
		{"prefix", "v=lk1 tcp=rpc.example.com https=https://login.example.com/prefix/", "https://login.example.com/prefix", nil},
		{"host", "v=lk1 https=login.example.com", "https://login.example.com", nil},
		{"missing", "v=lk1 tcp=rpc.example.com", "", ErrNoBrowserEndpoint},
		{"wrong version", "v=lk2 https=login.example.com", "", ErrNoBrowserEndpoint},
		{"http", "v=lk1 https=http://login.example.com", "", ErrInvalidBrowserEndpoint},
		{"userinfo", "v=lk1 https=https://user@login.example.com", "", ErrInvalidBrowserEndpoint},
		{"query", "v=lk1 https=https://login.example.com/?next=bad", "", ErrInvalidBrowserEndpoint},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &browserTXT{records: []string{tc.record}}
			got, err := DiscoverBrowserEndpoint(context.Background(), r, "example.com")
			if got != tc.want || !errors.Is(err, tc.err) || r.name != "_linkkeys_apis.example.com" {
				t.Fatalf("%q %v (%s)", got, err, r.name)
			}
		})
	}
	for _, tc := range []struct{ dns, want error }{
		{&net.DNSError{IsNotFound: true}, ErrNoBrowserEndpoint},
		{context.DeadlineExceeded, context.DeadlineExceeded},
	} {
		_, err := DiscoverBrowserEndpoint(context.Background(), &browserTXT{err: tc.dns}, "example.com")
		if !errors.Is(err, tc.want) {
			t.Fatal(err)
		}
	}
}
