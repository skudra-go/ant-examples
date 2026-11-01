package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/skudra-go/ant"

	"github.com/skudra-go/ant-examples/blog/models"
	"github.com/skudra-go/ant-examples/blog/utils"
)

// Posts manages articles.
type Posts struct {
	db *models.AppDB
}

// NewPosts returns the service.
func NewPosts(db *models.AppDB) *Posts { return &Posts{db} }

// PostInput is what a caller provides for a new post.
type PostInput struct {
	AuthorID int64
	Title    string
	Body     string
	Tags     []string
}

// PostChanges holds what an edit changes. Nil means leave it alone.
type PostChanges struct {
	Title *string
	Body  *string
	Tags  []string
}

// detailed is the base of every query that returns a post to a caller. The
// database counts comments, and authors and tags load in one batched query each,
// so a page of posts costs the same as one post.
func (s *Posts) detailed() *ant.Query[models.Post] {
	return s.db.Posts.
		IncludeOne(&models.Posts.Author).
		Include(&models.Posts.Tags).
		With(&models.Posts.CommentCount, ant.CountRelated(&models.Posts.Comments))
}

// published narrows detailed to what the public can see.
func (s *Posts) published() *ant.Query[models.Post] {
	return s.detailed().Where(ant.IsNotNull(&models.Posts.PublishedAt))
}

// reload returns a post as callers see it, after a write.
func (s *Posts) reload(ctx context.Context, id int64) (*models.Post, error) {
	post, err := s.detailed().Where(ant.Eq(&models.Posts.ID, id)).First(ctx)

	return post, translate(err)
}

// List returns a page of published posts, newest first, optionally with one tag.
func (s *Posts) List(
	ctx context.Context,
	tag string,
	page, size int,
) (*ant.Page[models.Post], error) {
	return s.published().
		When(tag != "", func(q *ant.Query[models.Post]) *ant.Query[models.Post] {
			return q.Where(ant.Has(&models.Posts.Tags, ant.Eq(&models.Tags.Name, tag)))
		}).
		Order(ant.Col(&models.Posts.PublishedAt).Desc()).
		Page(ctx, page, size)
}

// Get returns a published post with its comments, oldest first.
func (s *Posts) Get(ctx context.Context, slug string) (*models.Post, error) {
	post, err := s.published().
		Where(ant.Eq(&models.Posts.Slug, slug)).
		Include(
			&models.Posts.Comments,
			func(q *ant.Query[models.Comment]) *ant.Query[models.Comment] {
				return q.OrderBy(&models.Comments.CreatedAt)
			},
		).
		First(ctx)

	return post, translate(err)
}

// Create writes a draft and its tags in one transaction, so a failure leaves no
// half-made post.
func (s *Posts) Create(ctx context.Context, input PostInput) (*models.Post, error) {
	slug := utils.Slug(input.Title)
	if slug == "" {
		return nil, fmt.Errorf("%w: the title needs at least one letter or digit", ErrInvalid)
	}

	post := &models.Post{
		AuthorID: input.AuthorID,
		Title:    input.Title,
		Slug:     slug,
		Body:     input.Body,
	}

	err := s.db.Tx(ctx, func(tx *models.AppDB) error {
		if err := tx.Posts.Insert(ctx, post); err != nil {
			return err
		}

		tags, err := findOrCreateTags(ctx, tx, input.Tags)
		if err != nil {
			return err
		}

		return ant.Link(ctx, tx, post, &models.Posts.Tags, tags...)
	})

	switch {
	case ant.IsUniqueViolation(err):
		return nil, fmt.Errorf("%w: a post called %q", ErrConflict, slug)
	case ant.IsForeignKeyViolation(err):
		return nil, fmt.Errorf("%w: no user %d", ErrInvalid, input.AuthorID)
	case err != nil:
		return nil, err
	}

	return s.reload(ctx, post.ID)
}

// Update edits a post. This is the unit of work: load, change the struct, save. A
// session keeps this request's tracking apart from every other, and SaveChanges
// writes only the columns that changed. The shared context has tracking off, so
// the session asks for it.
func (s *Posts) Update(
	ctx context.Context,
	slug string,
	changes PostChanges,
) (*models.Post, error) {
	session := ant.Session(s.db, ant.WithTracking(true))

	var id int64

	err := session.Tx(ctx, func(tx *models.AppDB) error {
		post, err := tx.Posts.Where(ant.Eq(&models.Posts.Slug, slug)).First(ctx)
		if err != nil {
			return err
		}

		id = post.ID

		if changes.Title != nil {
			post.Title = *changes.Title
		}

		if changes.Body != nil {
			post.Body = *changes.Body
		}

		if changes.Tags != nil {
			tags, err := findOrCreateTags(ctx, tx, changes.Tags)
			if err != nil {
				return err
			}

			if err := ant.SyncLinks(ctx, tx, post, &models.Posts.Tags, tags...); err != nil {
				return err
			}
		}

		return tx.SaveChanges(ctx)
	})
	if err != nil {
		return nil, translate(err)
	}

	return s.reload(ctx, id)
}

// Publish makes a draft public. It is one UPDATE, with no read first, and the
// condition means publishing twice keeps the first date.
func (s *Posts) Publish(ctx context.Context, slug string) error {
	n, err := s.db.Posts.
		Where(ant.Eq(&models.Posts.Slug, slug), ant.IsNull(&models.Posts.PublishedAt)).
		Update(ctx, ant.Set(&models.Posts.PublishedAt, new(time.Now().UTC())))
	if err != nil || n > 0 {
		return err
	}

	_, err = s.db.Posts.Where(ant.Eq(&models.Posts.Slug, slug)).First(ctx)

	return translate(err)
}

// Delete hides a post. The model has a DeletedAt field, so this marks the row, and
// every query above skips it from now on.
func (s *Posts) Delete(ctx context.Context, slug string) error {
	n, err := s.db.Posts.Where(ant.Eq(&models.Posts.Slug, slug)).Delete(ctx)
	if err == nil && n == 0 {
		return ErrNotFound
	}

	return err
}

// Stats reads the post_stats view, as any other table.
func (s *Posts) Stats(ctx context.Context) ([]*models.PostStat, error) {
	return s.db.PostStats.OrderBy(&models.PostStats.Tag).All(ctx)
}

func findOrCreateTags(
	ctx context.Context,
	tx *models.AppDB,
	names []string,
) ([]*models.Tag, error) {
	tags := make([]*models.Tag, 0, len(names))

	for _, name := range names {
		name = utils.Slug(name)

		tag, _, err := tx.Tags.FirstOrCreate(
			ctx,
			&models.Tag{Name: name},
			ant.Eq(&models.Tags.Name, name),
		)
		if err != nil {
			return nil, err
		}

		tags = append(tags, tag)
	}

	return tags, nil
}

// translate turns ant's not-found error into this package's, so that routes need
// not know about ant.
func translate(err error) error {
	if errors.Is(err, ant.ErrNotFound) {
		return ErrNotFound
	}

	return err
}
