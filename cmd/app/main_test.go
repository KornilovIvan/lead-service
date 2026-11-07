package main

import (
	"testing"
	"time"
)

func TestValidateReaperAfter(t *testing.T) {
	tests := []struct {
		name    string
		after   time.Duration
		timeout time.Duration
		wantErr bool
	}{
		{"equal", 5 * time.Second, 5 * time.Second, true},
		{"less", 4 * time.Second, 5 * time.Second, true},
		{"greater", 6 * time.Second, 5 * time.Second, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateReaperAfter(tt.after, tt.timeout)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
