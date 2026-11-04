package service

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

func nullIfEmpty(v *string) *string {
	if v == nil || *v == "" {
		return nil
	}
	return v
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}
