package service

import (
	"errors"
	"testing"

	"lead-service/internal/model"
)

func TestValidateFields(t *testing.T) {
	empty := ""
	phone := "+79001234567"

	tests := []struct {
		name    string
		fn      func() error
		wantErr error
	}{
		{
			name: "empty phone",
			fn: func() error {
				_, err := validateLead(model.Lead{Phone: &empty, Status: "new"})
				return err
			},
			wantErr: ErrPhoneRequired,
		},
		{
			name: "missing phone",
			fn: func() error {
				_, err := validateLead(model.Lead{Status: "new"})
				return err
			},
			wantErr: ErrPhoneRequired,
		},
		{
			name: "status new",
			fn: func() error {
				_, err := validateLead(model.Lead{Phone: &phone, Status: "new"})
				return err
			},
		},
		{
			name: "status nope",
			fn: func() error {
				_, err := validateLead(model.Lead{Phone: &phone, Status: "nope"})
				return err
			},
			wantErr: ErrInvalidStatus,
		},
		{
			name: "customer name spaces",
			fn: func() error {
				_, err := validateCustomerName("   ")
				return err
			},
			wantErr: ErrNameRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got %v, want %v", err, tt.wantErr)
			}
		})
	}
}
