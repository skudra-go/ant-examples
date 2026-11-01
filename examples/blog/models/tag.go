package models

import "github.com/skudra-go/ant"

// Tags is the field probe for Tag.
var Tags = ant.Fields[Tag]()

// Tag groups posts.
type Tag struct {
	ID    int64  `json:"id"`
	Name  string `json:"name" ant:"unique"`
	Posts []Post `json:"-"`
}

// Configure says what the struct cannot.
func (Tag) Configure(b *ant.ModelBuilder[Tag]) {
	b.ManyToMany(&Tags.Posts).Through("post_tags")
}
