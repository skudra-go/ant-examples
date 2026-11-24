package models

import "github.com/skudra-go/ant"

type DB struct {
	ant.Context
	Users ant.Table[User]
}
