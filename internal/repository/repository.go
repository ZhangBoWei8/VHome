package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"
)

var (
	ErrNotFound        = errors.New("repository: resource not found")
	ErrConflict        = errors.New("repository: resource conflict")
	ErrInvalidArgument = errors.New("repository: invalid argument")
)

type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)

	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)

	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type Repository struct {
	db *sql.DB
	q  DBTX
}

func New(db *sql.DB) *Repository {
	return &Repository{
		db: db,
		q:  db,
	}
}

func (r *Repository) WithinTransaction(ctx context.Context, fn func(txRepository *Repository) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	txRepository := &Repository{
		db: r.db,
		q:  tx,
	}

	if err := fn(txRepository); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func isDuplicateKey(err error) bool {
	var mysqlErr *mysql.MySQLError

	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
