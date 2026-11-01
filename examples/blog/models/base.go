// Package models holds the database's structs and the context that ties them
// together. Everything that touches the database imports it, and it imports
// nothing from this project, so the migrate command builds without the web server.
//
// Relations point both ways, as User.Posts and Post.Author do, and Go forbids
// import cycles, so all models live in this one package, in a file each.
package models

import "time"

// Base is what every table starts with. ant supplies ant.Base, but this API wants
// its own JSON names, and any embedded struct works: the ID, CreatedAt, and
// UpdatedAt fields are found by name.
type Base struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
