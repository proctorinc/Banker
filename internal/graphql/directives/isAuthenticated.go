package directives

import (
	"context"
	"fmt"
	"log"

	"github.com/99designs/gqlgen/graphql"
	"github.com/proctorinc/banker/internal/auth"
)

func IsAuthenticated(ctx context.Context, obj interface{}, next graphql.Resolver) (res interface{}, err error) {
	user := auth.GetCurrentUser(ctx)

	if user == nil {
		log.Printf("IsAuthenticated? No")
		return nil, fmt.Errorf("You must be authenticated to request this endpoint")
	}

	log.Printf("IsAuthenticated? Yes: %s", user.ID)
	return next(ctx)
}