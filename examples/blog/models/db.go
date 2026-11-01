package models

import "github.com/skudra-go/ant"

// AppDB is the database. Each Table field is a table migrations manage. Models
// reached through relations are included without being listed here.
type AppDB struct {
	ant.Context
	Users    ant.Table[User]
	Posts    ant.Table[Post]
	Comments ant.Table[Comment]
	Tags     ant.Table[Tag]

	// PostStats is a view, so it has no table to manage.
	PostStats ant.Table[PostStat]
}
