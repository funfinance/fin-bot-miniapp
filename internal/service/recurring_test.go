package service

import (
	"testing"
	"time"
)

func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 12, 0, 0, 0, time.Local)
}

func TestNextWeeklyTrigger(t *testing.T) {
	tests := []struct {
		name     string
		days     string
		after    time.Time
		wantDate string
	}{
		{
			name:     "next weekday in same week",
			days:     "1,3,5", // Mon, Wed, Fri
			after:    date(2024, time.January, 1), // Monday
			wantDate: "2024-01-03",                // Wednesday
		},
		{
			name:     "wrap to next week",
			days:     "1,3,5",
			after:    date(2024, time.January, 5), // Friday
			wantDate: "2024-01-08",                // next Monday
		},
		{
			name:     "single day wraps to next week",
			days:     "3", // Wednesday only
			after:    date(2024, time.January, 3), // Wednesday
			wantDate: "2024-01-10",                // next Wednesday
		},
		{
			name:     "sunday wraps correctly",
			days:     "0,6", // Sun, Sat
			after:    date(2024, time.January, 6), // Saturday
			wantDate: "2024-01-07",                // Sunday
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := nextTrigger("weekly", tt.days, tt.after)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Format("2006-01-02") != tt.wantDate {
				t.Errorf("got %s, want %s", got.Format("2006-01-02"), tt.wantDate)
			}
		})
	}
}

func TestNextMonthlyTrigger(t *testing.T) {
	tests := []struct {
		name     string
		days     string
		after    time.Time
		wantDate string
	}{
		{
			name:     "next day in same month",
			days:     "1,15",
			after:    date(2024, time.January, 1),
			wantDate: "2024-01-15",
		},
		{
			name:     "wrap to next month",
			days:     "1,15",
			after:    date(2024, time.January, 15),
			wantDate: "2024-02-01",
		},
		{
			name:     "wrap to next year",
			days:     "1,15",
			after:    date(2024, time.December, 15),
			wantDate: "2025-01-01",
		},
		{
			name:     "day 31 clamped in Feb",
			days:     "31",
			after:    date(2024, time.January, 31),
			wantDate: "2024-02-29", // 2024 is leap year
		},
		{
			name:     "day 31 clamped in non-leap Feb",
			days:     "31",
			after:    date(2023, time.January, 31),
			wantDate: "2023-02-28",
		},
		{
			name:     "day 31 clamped in April",
			days:     "31",
			after:    date(2024, time.March, 31),
			wantDate: "2024-04-30",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := nextTrigger("monthly", tt.days, tt.after)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Format("2006-01-02") != tt.wantDate {
				t.Errorf("got %s, want %s", got.Format("2006-01-02"), tt.wantDate)
			}
		})
	}
}

func TestNextTriggerUnknownFrequency(t *testing.T) {
	_, err := nextTrigger("daily", "1", time.Now())
	if err == nil {
		t.Error("expected error for unknown frequency")
	}
}

func TestParseDaysInvalid(t *testing.T) {
	_, err := parseDays("1,abc,3")
	if err == nil {
		t.Error("expected error for invalid day value")
	}

	_, err = parseDays("")
	if err == nil {
		t.Error("expected error for empty days")
	}
}
