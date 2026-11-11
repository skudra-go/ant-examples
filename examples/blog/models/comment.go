package models

import "github.com/skudra-go/ant"

// Comments is the probe for Comment.
var Comments = ant.Fields[Comment]()

// Comment is a reader's reply to a post.
type Comment struct {
	Base
	PostID     int64  `json:"postId"`
	AuthorName string `json:"authorName"`
	Body       string `json:"body"`
	Post       *Post  `json:"-"`
}

// Configure says what the struct cannot.
func (Comment) Configure(b *ant.ModelBuilder[Comment]) {
	b.Check("comments_body_present", "body <> ''")
}
