package router

import (
	"errors"

	"azugo.io/azugo"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant-examples/playground/models"
)

const ErrUserExists = "user already exists"

func (r *router) GetUsers(ctx *azugo.Context) {
	users, err := r.Services().Users.GetUsers(ctx)
	if err != nil {
		ctx.Error(err)

		return
	}

	ctx.JSON(users)
}

func (r *router) GetUser(ctx *azugo.Context) {
	user, err := r.Services().Users.GetUserByID(ctx, ctx.Params.String("id"))
	if errors.Is(err, ant.ErrNotFound) {
		ctx.NotFound()

		return
	}

	if err != nil {
		ctx.Error(err)

		return
	}

	ctx.JSON(user)
}

func (r *router) CreateUser(ctx *azugo.Context) {
	user := &models.User{}
	if err := ctx.Body.JSON(user); err != nil {
		ctx.Error(err)

		return
	}

	if err := r.Services().Users.CreateUser(ctx, user); err != nil {
		if err.Error() == ErrUserExists {
			ctx.StatusCode(409)
			ctx.JSON(map[string]string{"error": ErrUserExists})

			return
		}

		ctx.Error(err)

		return
	}

	ctx.JSON(user)
}
