package auth

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestDelegationScopeCoverage(t *testing.T) {
	raw, err := os.ReadFile("../../../csil/longhouse.csil")
	if err != nil {
		t.Fatal(err)
	}
	excluded := map[string]bool{"auth.Login": true, "auth.Complete": true, "auth.Refresh": true, "auth.Logout": true, "auth.ResumeSession": true, "auth.EndSession": true, "auth.BeginCliLogin": true, "auth.InspectCliLogin": true, "auth.ApproveCliLogin": true, "auth.ExchangeCliLogin": true, "auth.RefreshSession": true, "auth.GetActAsScopeSet": true, "auth.ExchangeActAsCredential": true, "devauth.ListDevUsers": true, "devauth.DevLogin": true}
	services := regexp.MustCompile(`(?s)service (\w+)Service \{(.*?)\n\}`).FindAllStringSubmatch(string(raw), -1)
	ops := regexp.MustCompile(`(?m)^\s+([a-z][a-z-]+):`)
	for _, svc := range services {
		for _, op := range ops.FindAllStringSubmatch(svc[2], -1) {
			parts := strings.Split(op[1], "-")
			for i := range parts {
				parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
			}
			key := strings.ToLower(svc[1]) + "." + strings.Join(parts, "")
			scope, covered := DelegationMethodScopes[key]
			if !covered && !excluded[key] {
				t.Errorf("operation needs delegation policy: %s", key)
			}
			if covered && (excluded[key] || DelegationScopes[scope] == "") {
				t.Errorf("bad policy: %s", key)
			}
		}
	}
}

func TestDelegationPermissions(t *testing.T) {
	for _, tc := range []struct {
		scopes          []string
		service, method string
		allowed         bool
	}{
		{[]string{"tasks.read"}, "task", "ListTasks", true},
		{[]string{"tasks.read"}, "task", "CreateTask", false},
		{[]string{"tasks.write"}, "project", "CreateProject", false},
		{[]string{"projects.write"}, "project", "CreateProject", true},
		{[]string{"longhouse.full"}, "house", "DeleteHouse", true},
		{[]string{"longhouse.full"}, "auth", "Refresh", false},
		{[]string{"longhouse.full"}, "auth", "ApproveCliLogin", false},
		{[]string{"longhouse.full"}, "new", "FutureOperation", false},
	} {
		id := Identity{Delegation: &Delegation{Scopes: tc.scopes}}
		if got := id.AllowsDelegatedMethod(tc.service, tc.method); got != tc.allowed {
			t.Errorf("%v %s.%s: %v", tc.scopes, tc.service, tc.method, got)
		}
	}
}

func TestDelegationGrantExpiryBoundsMint(t *testing.T) {
	secret := []byte("secret")
	expires := time.Now().Add(30 * time.Second).Unix()
	token, err := Mint(secret, Identity{Delegation: &Delegation{GrantExpiresAt: expires}}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	id, err := Verify(secret, token)
	if err != nil {
		t.Fatal(err)
	}
	if id.ExpiresAt != expires {
		t.Fatalf("bearer outlives grant: %d", id.ExpiresAt)
	}
	if _, err := Mint(secret, Identity{Delegation: &Delegation{GrantExpiresAt: time.Now().Add(-time.Second).Unix()}}, time.Hour); err == nil {
		t.Fatal("expired grant minted bearer")
	}
}
