package service

import (
	"errors"
	"testing"
)

func TestNormalizePhoneAndEmail(t *testing.T) {
	tests := []struct {
		name          string
		phone         *string
		email         *string
		wantPhone     string
		wantEmail     string
		wantEmailNull bool
		wantErr       error
	}{
		{
			name:      "8 (999) 123-45-67",
			phone:     strPtr("8 (999) 123-45-67"),
			wantPhone: "79991234567",
		},
		{
			name:      "+7 999 123-45-67",
			phone:     strPtr("+7 999 123-45-67"),
			wantPhone: "79991234567",
		},
		{
			name:    "123",
			phone:   strPtr("123"),
			wantErr: ErrInvalidPhone,
		},
		{
			name:      "email A@X.ru",
			email:     strPtr("A@X.ru"),
			wantEmail: "a@x.ru",
		},
		{
			name:          "email spaces",
			email:         strPtr("   "),
			wantEmailNull: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.email != nil {
				got := normalizeEmail(tt.email)
				if tt.wantEmailNull {
					if got != nil {
						t.Fatalf("got %q, want NULL", *got)
					}
					return
				}
				if got == nil {
					t.Fatal("got NULL, want email")
				}
				if *got != tt.wantEmail {
					t.Fatalf("got %q, want %q", *got, tt.wantEmail)
				}
				return
			}

			got, err := normalizePhone(tt.phone)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got err %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if got == nil {
				t.Fatal("got nil phone")
			}
			if *got != tt.wantPhone {
				t.Fatalf("got %q, want %q", *got, tt.wantPhone)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}
