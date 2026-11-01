package responses

import "github.com/skudra-go/ant-examples/blog/models"

// User is a user with their published post count.
type User struct {
	models.User
	PostCount int64 `json:"postCount"`
}
