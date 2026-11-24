package router

import (
	"net/http"

	"azugo.io/azugo"

	"github.com/skudra-go/ant-examples/playground/models"
)

// Handlers pass every error to ctx.Error: the app's error handler picks the
// status, so a handler never has to know which errors mean what.

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
		ctx.Error(err)

		return
	}

	ctx.StatusCode(http.StatusCreated)
	ctx.JSON(user)
}
