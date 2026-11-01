package models

import (
	"time"

	"github.com/skudra-go/ant"
)

// Posts is the field probe for Post.
var Posts = ant.Fields[Post]()

// Post is an article. Soft deleted, since comments and links outlive a change of
// heart. PublishedAt is nil for a draft.
type Post struct {
	Base
	AuthorID    int64         `json:"authorId"`
	Title       string        `json:"title"`
	Slug        string        `json:"slug" ant:"unique"`
	Body        string        `json:"body"`
	PublishedAt *time.Time    `json:"publishedAt,omitempty"`
	DeletedAt   ant.DeletedAt `json:"-"`

	Author   *User     `json:"-"`
	Tags     []Tag     `json:"-"`
	Comments []Comment `json:"-"`

	// CommentCount has no column. Queries fill it with ant.CountRelated.
	CommentCount int64 `json:"commentCount" ant:"computed"`
}

// Configure says what the struct cannot.
func (Post) Configure(b *ant.ModelBuilder[Post]) {
	b.BelongsTo(&Posts.Author).ForeignKey(&Posts.AuthorID)
	b.HasMany(&Posts.Comments).OnDelete(ant.Cascade)
	b.ManyToMany(&Posts.Tags).Through("post_tags")
	b.Index(&Posts.PublishedAt).Desc().Where("deleted_at IS NULL").Name("posts_published_idx")
}
