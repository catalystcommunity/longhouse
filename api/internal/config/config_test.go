package config

import (
	"testing"
	"time"
)

func TestPositiveSeconds(t *testing.T) {
	duration, err := positiveSeconds("TEST", "43200")
	if err != nil {
		t.Fatalf("positiveSeconds: %v", err)
	}
	if duration != 12*time.Hour {
		t.Fatalf("duration: got %v, want 12h", duration)
	}
	for _, raw := range []string{"", "zero", "0", "-1"} {
		if _, err := positiveSeconds("TEST", raw); err == nil {
			t.Errorf("positiveSeconds(%q) accepted an invalid value", raw)
		}
	}
}
