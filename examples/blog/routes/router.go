// Package routes is the HTTP layer. Handlers read a request, call a service, and
// write a response, and leave every decision to the service.
package routes

import (
	"net/http"

	blog "github.com/skudra-go/ant-examples/blog"
	"github.com/skudra-go/ant-examples/blog/middleware"
)

// New returns the API.
func New(app *blog.App) http.Handler {
	mux := http.NewServeMux()
	admin := middleware.RequireToken(app.Config.AdminToken)
	protected := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, admin(handler))
	}

	mux.HandleFunc("GET /healthz", healthz(app))

	posts := postRoutes{app.Services.Posts, app.Services.Comments}
	mux.HandleFunc("GET /posts", posts.list)
	mux.HandleFunc("GET /posts/{slug}", posts.get)
	mux.HandleFunc("GET /stats", posts.stats)
	mux.HandleFunc("POST /posts/{slug}/comments", posts.comment)
	protected("POST /posts", posts.create)
	protected("PATCH /posts/{slug}", posts.update)
	protected("POST /posts/{slug}/publish", posts.publish)
	protected("DELETE /posts/{slug}", posts.delete)

	users := userRoutes{app.Services.Users}
	mux.HandleFunc("GET /users/{id}", users.get)
	protected("POST /users", users.create)

	handler := http.Handler(mux)
	handler = middleware.Logging(app.Log)(handler)

	return middleware.Recover(app.Log)(handler)
}
