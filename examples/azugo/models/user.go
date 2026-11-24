package models

import (
	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/ids"
	"github.com/skudra-go/ant/validate"
)

type User struct {
	Base

	Name  string
	Email string `ant:"unique"`
}

var Users = ant.Fields[User]()

func (u *User) Configure(b *ant.ModelBuilder[User]) {
	ant.GenerateKey(b, &Users.ID, ids.UUID)

	b.Validate(&Users.Email, validate.Email)
}
