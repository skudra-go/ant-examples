package models

import "github.com/skudra-go/ant"

// Users is the field probe for User: &models.Users.Email names the email column.
var Users = ant.Fields[User]()

// User writes posts.
type User struct {
	Base
	Name  string  `json:"name"`
	Email string  `json:"email" ant:"unique"`
	About *string `json:"about,omitempty"`
	Posts []Post  `json:"-"`
}
