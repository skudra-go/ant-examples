package routes

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/skudra-go/ant-examples/blog/routes/requests"
	"github.com/skudra-go/ant-examples/blog/routes/responses"
	"github.com/skudra-go/ant-examples/blog/services"
)

type userRoutes struct {
	users *services.Users
}

func (u userRoutes) get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, fmt.Errorf("%w: the id must be a number", services.ErrInvalid))

		return
	}

	user, posts, err := u.users.Get(r.Context(), id)
	if err != nil {
		fail(w, err)

		return
	}

	writeJSON(w, http.StatusOK, responses.User{User: *user, PostCount: posts})
}

func (u userRoutes) create(w http.ResponseWriter, r *http.Request) {
	body, ok := read[requests.CreateUser](w, r)
	if !ok {
		return
	}

	user, err := u.users.Create(r.Context(), body.Name, body.Email)
	if err != nil {
		fail(w, err)

		return
	}

	writeJSON(w, http.StatusCreated, user)
}
