// Package responses holds the shapes the API returns. Each embeds its model and
// adds what a client needs, so a new column appears in responses by itself, and a
// field tagged json:"-" on the model, such as a relation, stays out.
package responses

import (
	"slices"

	"github.com/skudra-go/ant"

	"github.com/skudra-go/ant-examples/blog/models"
)

// Author is the public part of a user.
type Author struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// Post is a post in a list.
type Post struct {
	models.Post
	Author *Author  `json:"author,omitempty"`
	Tags   []string `json:"tags"`
}

// PostDetail is a post with its comments.
type PostDetail struct {
	Post
	Comments []models.Comment `json:"comments"`
}

// Posts is a page of posts.
type Posts struct {
	Items []Post `json:"items"`
	Total int64  `json:"total"`
	Page  int    `json:"page"`
	Size  int    `json:"size"`
}

// NewPost shapes a post.
func NewPost(post *models.Post) Post {
	out := Post{Post: *post, Tags: []string{}}

	if post.Author != nil {
		out.Author = &Author{ID: post.Author.ID, Name: post.Author.Name}
	}

	for _, tag := range post.Tags {
		out.Tags = append(out.Tags, tag.Name)
	}

	slices.Sort(out.Tags)

	return out
}

// NewPostDetail shapes a post and its comments.
func NewPostDetail(post *models.Post) PostDetail {
	comments := post.Comments
	if comments == nil {
		comments = []models.Comment{}
	}

	return PostDetail{Post: NewPost(post), Comments: comments}
}

// NewPosts shapes a page.
func NewPosts(page *ant.Page[models.Post]) Posts {
	out := Posts{
		Items: make([]Post, len(page.Items)),
		Total: page.Total,
		Page:  page.Number,
		Size:  page.Size,
	}

	for i, post := range page.Items {
		out.Items[i] = NewPost(post)
	}

	return out
}
