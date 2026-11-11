# hello

ant in one file, on SQLite. Needs Go 1.27 or later.

```sh
go run .
```

```text
Created Alice
Alice is 30 and has 2 posts
Happy birthday, Alice
```

It creates `hello.db`, saves a user with two posts, loads them back and saves a changed age with `SaveChanges`. Run it again and Alice is a year older. Delete `hello.db` to start over.

The tables are plain SQL because migrations are PostgreSQL only for now. [blog](../blog/) shows migrations.

To start your own project from it:

```sh
mkdir hello && cd hello
go mod init example.com/hello
# save main.go from this folder here
go mod tidy
go run .
```
