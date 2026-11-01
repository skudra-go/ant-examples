package services

import (
	"context"
	"fmt"

	"github.com/skudra-go/ant"

	"github.com/skudra-go/ant-examples/blog/models"
)

// Comments manages replies.
type Comments struct {
	db *models.AppDB
}

// NewComments returns the service.
func NewComments(db *models.AppDB) *Comments { return &Comments{db} }

// Add replies to a published post.
func (s *Comments) Add(ctx context.Context, slug, author, body string) (*models.Comment, error) {
	if body == "" {
		return nil, fmt.Errorf("%w: a comment needs a body", ErrInvalid)
	}

	post, err := s.db.Posts.
		Where(ant.Eq(&models.Posts.Slug, slug), ant.IsNotNull(&models.Posts.PublishedAt)).
		Column(&models.Posts.ID).
		First(ctx)
	if err != nil {
		return nil, translate(err)
	}

	comment := &models.Comment{PostID: post.ID, AuthorName: author, Body: body}

	return comment, s.db.Comments.Insert(ctx, comment)
}
