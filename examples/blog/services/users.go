package services

import (
	"context"
	"fmt"

	"github.com/skudra-go/ant"

	"github.com/skudra-go/ant-examples/blog/models"
)

// Users manages authors.
type Users struct {
	db *models.AppDB
}

// NewUsers returns the service.
func NewUsers(db *models.AppDB) *Users { return &Users{db} }

// Get returns a user and how many posts they have published.
func (s *Users) Get(ctx context.Context, id int64) (*models.User, int64, error) {
	user, err := s.db.Users.Find(ctx, id)
	if err != nil {
		return nil, 0, translate(err)
	}

	posts, err := s.db.Posts.
		Where(ant.Eq(&models.Posts.AuthorID, id), ant.IsNotNull(&models.Posts.PublishedAt)).
		Count(ctx)

	return user, posts, err
}

// Create adds a user.
func (s *Users) Create(ctx context.Context, name, email string) (*models.User, error) {
	user := &models.User{Name: name, Email: email}

	err := s.db.Users.Insert(ctx, user)
	if ant.IsUniqueViolation(err) {
		return nil, fmt.Errorf("%w: a user with email %s", ErrConflict, email)
	}

	return user, err
}
