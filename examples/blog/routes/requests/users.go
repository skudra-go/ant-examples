package requests

// CreateUser is the body of POST /users.
type CreateUser struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}
