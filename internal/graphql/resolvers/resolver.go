package resolvers

import (
	"github.com/proctorinc/banker/internal/auth"
	"github.com/proctorinc/banker/internal/db"
	gen "github.com/proctorinc/banker/internal/graphql/generated"
)

type Resolver struct {
	Repository  db.Repository
	AuthService auth.AuthService
	// DataLoaders dataloaders.Retriever
}

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
type accountResolver struct{ *Resolver }
type transactionResolver struct{ *Resolver }
type userResolver struct{ *Resolver }

func (r *Resolver) Mutation() gen.MutationResolver {
	return &mutationResolver{r}
}

func (r *Resolver) Query() gen.QueryResolver {
	return &queryResolver{r}
}

func (r *Resolver) User() gen.UserResolver {
	return &userResolver{r}
}

func (r *Resolver) Account() gen.AccountResolver {
	return &accountResolver{r}
}

func (r *Resolver) Transaction() gen.TransactionResolver {
	return &transactionResolver{r}
}