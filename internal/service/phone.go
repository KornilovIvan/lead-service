package service

import (
	"strings"
	"unicode"
)

func normalizePhone(v *string) (*string, error) {
	if v == nil {
		return nil, ErrPhoneRequired
	}
	if strings.TrimSpace(*v) == "" {
		return nil, ErrPhoneRequired
	}

	digits := digitsOnly(*v)
	switch {
	case len(digits) == 11 && digits[0] == '8':
		digits = "7" + digits[1:]
	case len(digits) == 10:
		digits = "7" + digits
	}

	if len(digits) != 11 || digits[0] != '7' {
		return nil, ErrInvalidPhone
	}
	return &digits, nil
}

func normalizeEmail(v *string) *string {
	if v == nil {
		return nil
	}
	email := strings.ToLower(strings.TrimSpace(*v))
	if email == "" {
		return nil
	}
	return &email
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
