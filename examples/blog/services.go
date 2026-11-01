package blog

import (
	"github.com/skudra-go/ant-examples/blog/models"
	"github.com/skudra-go/ant-examples/blog/services"
)

// Services holds the business logic. Routes call these, and only these touch the
// database.
type Services struct {
	Posts    *services.Posts
	Comments *services.Comments
	Users    *services.Users
}

func newServices(db *models.AppDB) Services {
	return Services{
		Posts:    services.NewPosts(db),
		Comments: services.NewComments(db),
		Users:    services.NewUsers(db),
	}
}
