// Package requests holds the shapes the API accepts. Each is separate from its
// model, so a client can only set what is listed here.
package requests

// CreatePost is the body of POST /posts.
type CreatePost struct {
	AuthorID int64    `json:"authorId"`
	Title    string   `json:"title"`
	Body     string   `json:"body"`
	Tags     []string `json:"tags"`
}

// UpdatePost is the body of PATCH /posts/{slug}. A missing field stays as it is.
type UpdatePost struct {
	Title *string  `json:"title"`
	Body  *string  `json:"body"`
	Tags  []string `json:"tags"`
}

// CreateComment is the body of POST /posts/{slug}/comments.
type CreateComment struct {
	AuthorName string `json:"authorName"`
	Body       string `json:"body"`
}
