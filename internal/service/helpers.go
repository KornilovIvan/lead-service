package service

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

func requirePhone(v *string) (*string, error) {
	if v == nil {
		return nil, ErrPhoneRequired
	}
	phone := strings.TrimSpace(*v)
	if phone == "" {
		return nil, ErrPhoneRequired
	}
	return &phone, nil
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}
