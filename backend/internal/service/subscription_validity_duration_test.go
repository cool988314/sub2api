package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAddValidityDays_UsesExact24HoursAcrossDST(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("timezone data unavailable")
	}

	// 2026-03-08 is DST start day in New York.
	base := time.Date(2026, 3, 8, 1, 30, 0, 0, loc)
	got := addValidityDays(base, 1)
	calendar := base.AddDate(0, 0, 1)

	calendarDiff := calendar.Sub(base)
	if calendarDiff == 24*time.Hour {
		t.Skip("selected timezone/date does not cross DST on this system")
	}

	require.Equal(t, 24*time.Hour, got.Sub(base))
	require.NotEqual(t, calendar, got)
}

func TestAddValidityDays_NegativeDays(t *testing.T) {
	base := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	got := addValidityDays(base, -1)
	require.Equal(t, -24*time.Hour, got.Sub(base))
}
