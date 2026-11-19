package app

import (
	"testing"
	"time"
)

func TestClockAndPlays(t *testing.T) {
	if s := clock(185 * time.Second); s != "3:05" {
		t.Error(s)
	}
	// A podcast's program lasts hours, and the list says so.
	if s := clockLong(2*time.Hour + 14*time.Minute + 5*time.Second); s != "2:14:05" {
		t.Error(s)
	}
	if s := clockLong(185 * time.Second); s != "3:05" {
		t.Error(s)
	}
	if s := plays(123456789); s != "1.2亿" {
		t.Error(s)
	}
	if s := plays(56789); s != "6万" {
		t.Error(s)
	}
}
