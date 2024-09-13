package resolvers

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/google/uuid"
	"github.com/proctorinc/banker/internal/auth"
	"github.com/proctorinc/banker/internal/chase"
	"github.com/proctorinc/banker/internal/db"
	"github.com/proctorinc/banker/internal/graphql/utils"
)

func (r *accountResolver) ID(ctx context.Context, account *db.Account) (uuid.UUID, error) {
	return account.ID, nil
}

func (r *accountResolver) SourceId(ctx context.Context, account *db.Account) (string, error) {
	masked := utils.MaskData(account.Sourceid)
	return masked, nil
}

func (r *accountResolver) UploadSource(ctx context.Context, account *db.Account) (string, error) {
	return string(account.Uploadsource), nil
}

func (r *accountResolver) Type(ctx context.Context, account *db.Account) (string, error) {
	return string(account.Type), nil
}

func (r *accountResolver) Name(ctx context.Context, account *db.Account) (string, error) {
	return account.Name, nil
}

func (r *accountResolver) RoutingNumber(ctx context.Context, account *db.Account) (*string, error) {
	if len(account.Routingnumber.String) > 0 {
		masked := utils.MaskData(account.Routingnumber.String)
		return &masked, nil
	}

	return nil, nil
}

// Queries

func (r *queryResolver) Account(ctx context.Context, accountId uuid.UUID) (*db.Account, error) {
	user := auth.GetCurrentUser(ctx)
	account, err := r.Repository.GetAccount(ctx, db.GetAccountParams{
		ID:      accountId,
		Ownerid: user.ID,
	})

	if err != nil {
		return nil, err
	}

	return &account, nil
}

func (r *queryResolver) Accounts(ctx context.Context) ([]db.Account, error) {
	user := auth.GetCurrentUser(ctx)

	return r.Repository.ListAccounts(ctx, user.ID)
}

// Mutations

func (r *mutationResolver) ChaseOFXUpload(ctx context.Context, reader graphql.Upload) (bool, error) {
	accountsUploaded := 0
	transactionsUploaded := 0
	accountsFailed := 0
	transactionsFailed := 0

	if !bytes.HasSuffix([]byte(reader.Filename), []byte(".ofx")) &&
		!bytes.HasSuffix([]byte(reader.Filename), []byte(".QFX")) &&
		!bytes.HasSuffix([]byte(reader.Filename), []byte(".")) {
		log.Printf("Invalid extension: %s", reader.Filename)
		return false, fmt.Errorf("Invalid file extension. .OFX/.QBX/.QBO required")
	}

	user := auth.GetCurrentUser(ctx)
	ofxResult, err := chase.ParseChaseOFX(reader.File)

	if err != nil {
		return false, err
	}

	account, err := r.Repository.UpsertAccount(ctx, db.UpsertAccountParams{
		Sourceid:      ofxResult.Account.AccountId,
		Uploadsource:  db.UploadSourceCHASEOFXUPLOAD,
		Name:          ofxResult.Account.Name,
		Type:          db.AccountType(ofxResult.Account.Type),
		Routingnumber: sql.NullString{String: ofxResult.Account.BankId, Valid: len(ofxResult.Account.BankId) > 0},
		Ownerid:       user.ID,
	})

	if err != nil {
		return false, err
	}

	// Increment successful account upload
	accountsUploaded++

	for _, tx := range ofxResult.Transactions {
		_, err := r.Repository.UpsertTransaction(ctx, db.UpsertTransactionParams{
			Ownerid:         user.ID,
			Amount:          int32(tx.Amount * 100),
			Payeeid:         sql.NullString{String: tx.PayeeId, Valid: len(tx.PayeeId) > 0},
			Payee:           sql.NullString{String: tx.Payee, Valid: len(tx.Payee) > 0},
			Payeefull:       sql.NullString{String: tx.PayeeFull, Valid: len(tx.PayeeFull) > 0},
			Sourceid:        tx.Id,
			Uploadsource:    db.UploadSourceCHASEOFXUPLOAD,
			Isocurrencycode: ofxResult.Account.IsoCurrencyCode,
			Date:            tx.DatePosted,
			Description:     tx.Description,
			Type:            db.TransactionType(tx.Type),
			Updated:         time.Now(),
			Checknumber:     sql.NullString{String: tx.CheckNumber, Valid: len(tx.CheckNumber) > 0},
			Accountid:       account.ID,
		})

		// Increment successful transaction upload
		if err == nil {
			transactionsUploaded++
		} else {
			transactionsFailed++
			log.Println(tx)
			log.Println(err)
		}
	}

	log.Printf("Account(s) [updated:%d, failed:%d] Transaction(s) [updated:%d, failed:%d]", accountsUploaded, accountsFailed, transactionsUploaded, transactionsFailed)

	return true, nil
}