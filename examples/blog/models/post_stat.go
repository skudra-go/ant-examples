package models

import "github.com/skudra-go/ant"

// PostStats is the probe for PostStat.
var PostStats = ant.Fields[PostStat]()

// PostStat is a row of the post_stats view, which is defined in
// migrations/code/10_post_stats.sql. A view model reads like any other and refuses
// writes, and migrations leave it to that file.
type PostStat struct {
	Tag      string `json:"tag"`
	Posts    int64  `json:"posts"`
	Comments int64  `json:"comments"`
}

// Configure says what the struct cannot.
func (PostStat) Configure(b *ant.ModelBuilder[PostStat]) { b.View("post_stats") }
