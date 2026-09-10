package handler

import (
	"errors"
	"testing"
)

func TestOptionalDataSyncUnavailable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "python missing", err: errors.New("Python was not found in PATH; Data Sync needs Python 3.10+ and Git"), want: true},
		{name: "windows app alias", err: errors.New("Data Sync process failed: exit status 9009"), want: true},
		{name: "network error", err: errors.New("Data Sync process failed: upstream timeout"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := optionalDataSyncUnavailable(tt.err); got != tt.want {
				t.Fatalf("optionalDataSyncUnavailable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
