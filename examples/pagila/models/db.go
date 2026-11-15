// Package models describes the Pagila sample database as it already exists. The
// structs were written by hand from db/pagila-schema.sql, and `migrate drift` checks
// them against the live database.
package models

import (
	"github.com/skudra-go/ant"
)

// AppDB is the database: one table per Pagila table, plus the views it reads.
type AppDB struct {
	ant.Context

	Actors         ant.Table[Actor]
	Addresses      ant.Table[Address]
	Categories     ant.Table[Category]
	Cities         ant.Table[City]
	Countries      ant.Table[Country]
	Customers      ant.Table[Customer]
	Films          ant.Table[Film]
	FilmActors     ant.Table[FilmActor]
	FilmCategories ant.Table[FilmCategory]
	Inventories    ant.Table[Inventory]
	Languages      ant.Table[Language]
	Payments       ant.Table[Payment]
	Rentals        ant.Table[Rental]
	Staff          ant.Table[Staff]
	Stores         ant.Table[Store]

	CustomerList ant.Table[CustomerListRow]
	SalesByStore ant.Table[SalesByStoreRow]
}
