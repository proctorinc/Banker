package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

type Repository interface {
	// Users
	GetUser(ctx context.Context, id uuid.UUID) (User, error)
	GetUserByEmail(ctx context.Context, email string) (User, error)
	ListUsers(ctx context.Context) ([]User, error)
	CreateUser(ctx context.Context, arg CreateUserParams) (User, error)
	DeleteUser(ctx context.Context, id uuid.UUID) (User, error)

	// Accounts
	GetAccount(ctx context.Context, arg GetAccountParams) (Account, error)
	ListAccounts(ctx context.Context, ownerid uuid.UUID) ([]Account, error)
	UpsertAccount(ctx context.Context, arg UpsertAccountParams) (Account, error)

	// Transactions
	GetTransaction(ctx context.Context, arg GetTransactionParams) (Transaction, error)
	ListTransactions(ctx context.Context, ownerid uuid.UUID) ([]Transaction, error)
	UpsertTransaction(ctx context.Context, arg UpsertTransactionParams) (Transaction, error)
	DeleteTransaction(ctx context.Context, id uuid.UUID) (Transaction, error)
	GetTotalSpending(ctx context.Context, id uuid.UUID) (int64, error)
	GetTotalIncome(ctx context.Context, id uuid.UUID) (int64, error)
}

type repositoryService struct {
	*Queries
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &repositoryService{
		Queries: New(db),
		db:      db,
	}
}

func Open(dataSourceName string) (*sql.DB, error) {
	return sql.Open("postgres", dataSourceName)
}

func (r repositoryService) withTx(ctx context.Context, txFn func(*Queries) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	q := New(tx)
	err = txFn(q)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			err = fmt.Errorf("tx failed: %v, unable to rollback: %v", err, rbErr)
		}
	} else {
		err = tx.Commit()
	}
	return err
}
