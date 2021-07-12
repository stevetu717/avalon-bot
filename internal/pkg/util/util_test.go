package util

import (
	"testing"
	"time"
)

func TestDateTimeWithinTwoDays(t *testing.T) {
	type args struct {
		dateTime time.Time
	}
	today := time.Now().Local()
	today = time.Date(today.Year(), today.Month(), today.Day(), 23, 55, 0, 0, time.Local)
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
		{
			"Today",
			args{dateTime: today},
			true,
		},
		{
			"Tomorrow",
			args{dateTime: time.Now().Local().Add(24 * time.Hour)},
			true,
		},
		{
			"Day After tomorrow",
			args{dateTime: time.Now().Local().Add(48 * time.Hour)},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DateTimeWithinTwoDays(tt.args.dateTime); got != tt.want {
				t.Errorf("DateTimeWithinTwoDays() = %v, want %v", got, tt.want)
			}
		})
	}
}
