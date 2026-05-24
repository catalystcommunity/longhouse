package auth

import (
	"strings"
	"testing"
	"time"
)

func TestMintVerify_RoundTrip(t *testing.T) {
	secret := []byte("super-secret")
	in := Identity{
		Domain:      "todandlorna.com",
		UserID:      "tod",
		DisplayName: "Tod Hansmann",
	}
	tok, err := Mint(secret, in, time.Hour)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if strings.Count(tok, ".") != 2 {
		t.Errorf("expected 3 dot-separated parts, got %q", tok)
	}

	got, err := Verify(secret, tok)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got.Domain != "todandlorna.com" || got.UserID != "tod" {
		t.Errorf("identity mismatch: %+v", got)
	}
	if got.DisplayName != "Tod Hansmann" {
		t.Errorf("display name lost: %+v", got)
	}
	if got.IssuedAt == 0 || got.ExpiresAt == 0 || got.ExpiresAt <= got.IssuedAt {
		t.Errorf("timestamps look wrong: iat=%d exp=%d", got.IssuedAt, got.ExpiresAt)
	}
}

func TestVerify_TamperedPayload(t *testing.T) {
	secret := []byte("super-secret")
	tok, err := Mint(secret, Identity{Domain: "d", UserID: "u"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(tok, ".")
	// Flip one byte in the payload, keep the original signature.
	parts[1] = parts[1][:len(parts[1])-1] + "X"
	tampered := strings.Join(parts, ".")

	if _, err := Verify(secret, tampered); err != ErrBadSignature {
		t.Errorf("got %v, want ErrBadSignature", err)
	}
}

func TestVerify_WrongSecret(t *testing.T) {
	tok, err := Mint([]byte("alice"), Identity{Domain: "d", UserID: "u"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Verify([]byte("bob"), tok); err != ErrBadSignature {
		t.Errorf("got %v, want ErrBadSignature", err)
	}
}

func TestVerify_Malformed(t *testing.T) {
	for _, tok := range []string{"", "abc", "abc.def", "a.b.c.d"} {
		if _, err := Verify([]byte("k"), tok); err != ErrTokenMalformed {
			t.Errorf("Verify(%q) = %v, want ErrTokenMalformed", tok, err)
		}
	}
}

func TestVerify_Expired(t *testing.T) {
	secret := []byte("k")
	tok, err := Mint(secret, Identity{Domain: "d", UserID: "u"}, -time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(secret, tok); err != ErrTokenExpired {
		t.Errorf("got %v, want ErrTokenExpired", err)
	}
}

func TestMint_DefaultTTL(t *testing.T) {
	secret := []byte("k")
	tok, err := Mint(secret, Identity{Domain: "d", UserID: "u"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Verify(secret, tok)
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Unix(got.ExpiresAt, 0).Sub(time.Unix(got.IssuedAt, 0)); d != DefaultTTL {
		t.Errorf("default TTL: got %v, want %v", d, DefaultTTL)
	}
}

func TestMemberContext_HasRole(t *testing.T) {
	c := &MemberContext{MemberID: "m1", HouseID: "h1", Roles: []string{"admin", "member"}}
	if !c.HasRole("admin") || !c.HasRole("member") {
		t.Error("expected admin+member")
	}
	if c.HasRole("ghost") {
		t.Error("unexpected role")
	}
}
