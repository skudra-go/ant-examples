package router

import app "github.com/skudra-go/ant-examples/playground"

type router struct {
	*app.App
}

func RegisterRoutes(app *app.App) *app.App {
	r := &router{
		App: app,
	}

	app.Get("/users", r.GetUsers)
	app.Get("/users/{id}", r.GetUser)
	app.Post("/users", r.CreateUser)

	return app
}
