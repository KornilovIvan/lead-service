package service

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

func isForeignKeyViolation(err error) bool {
	return pgCode(err) == "23503"
}

func isUniqueViolation(err error) bool {
	return pgCode(err) == "23505"
}

func isActiveJobConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.ConstraintName == "jobs_one_active_per_lead"
}

func pgCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}
