package app

import (
	"github.com/skudra-go/ant-examples/playground/models"
	"github.com/skudra-go/ant-examples/playground/services"
)

type Services struct {
	Users *services.UserService
}

func newServices(db *models.DB) *Services {
	return &Services{
		Users: services.NewUserService(db),
	}
}
