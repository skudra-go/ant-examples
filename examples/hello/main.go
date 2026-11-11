// Command hello is ant in one file, on SQLite: nothing to install but Go.
package main

import (
	"context"
	"fmt"
	"log"

	_ "modernc.org/sqlite" // the "sqlite" driver, in pure Go

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/dialect/sqlite"
)

type User struct {
	ant.Base // ID, CreatedAt, UpdatedAt
	Name     string
	Email    string `ant:"unique"`
	Age      int
	Posts    []Post // found through Post.UserID
}

type Post struct {
	ant.Base
	UserID int64
	Title  string
}

type AppDB struct {
	ant.Context
	Users ant.Table[User]
	Posts ant.Table[Post]
}

var Users = ant.Fields[User]()

// Migrations run on PostgreSQL only for now, so on SQLite the tables are plain SQL.
const tables = `
CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY,
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,
	name TEXT NOT NULL,
	email TEXT NOT NULL UNIQUE,
	age INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS posts (
	id INTEGER PRIMARY KEY,
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,
	user_id INTEGER NOT NULL REFERENCES users (id),
	title TEXT NOT NULL
)`

func main() {
	ctx := context.Background()

	db, err := ant.Open[AppDB](ctx, sqlite.Dialect, "file:hello.db")
	if err != nil {
		log.Fatal(err)
	}

	defer db.Close()

	if _, err := ant.Raw[any](db, tables).Exec(ctx); err != nil {
		log.Fatal(err)
	}

	// Insert Alice and her posts on the first run, and find her on later ones.
	alice, created, err := db.Users.FirstOrCreate(
		ctx,
		&User{
			Name:  "Alice",
			Email: "alice@example.com",
			Age:   30,
			Posts: []Post{{Title: "Hello"}, {Title: "Second post"}},
		},
		ant.Eq(&Users.Email, "alice@example.com"),
	)
	if err != nil {
		log.Fatal(err)
	}

	if created {
		fmt.Println("Created", alice.Name)
	}

	// Load every adult with their posts. ant.Gte(&Users.Age, "18") wouldn't compile.
	adults, err := db.Users.
		Where(ant.Gte(&Users.Age, 18)).
		Include(&Users.Posts). // one query for every user's posts
		OrderBy(&Users.Name).
		All(ctx)
	if err != nil {
		log.Fatal(err)
	}

	for _, u := range adults {
		fmt.Printf("%s is %d and has %d posts\n", u.Name, u.Age, len(u.Posts))
	}

	// Change a loaded user and save. Only the age column is written.
	adults[0].Age++

	err = db.SaveChanges(ctx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Happy birthday,", adults[0].Name)
}
