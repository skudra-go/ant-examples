package models

import (
	"time"

	"github.com/skudra-go/ant"
)

// Country is the country table.
type Country struct {
	CountryID int32
	Country   string
	UpdatedAt time.Time `ant:"column=last_update,default=now()"`

	Cities []City
}

// City is the city table.
type City struct {
	CityID    int32
	City      string
	CountryID int32
	UpdatedAt time.Time `ant:"column=last_update,default=now()"`

	Country *Country
}

// Address is the address table.
type Address struct {
	AddressID  int32
	Address    string
	Address2   *string
	District   string
	CityID     int32
	PostalCode *string
	Phone      string
	UpdatedAt  time.Time `ant:"column=last_update,default=now()"`

	City *City
}

var (
	Countries = ant.Fields[Country]()
	Cities    = ant.Fields[City]()
	Addresses = ant.Fields[Address]()
)

func (Country) Configure(b *ant.ModelBuilder[Country]) {
	b.Table("country")
	b.Key(&Countries.CountryID)
}

func (City) Configure(b *ant.ModelBuilder[City]) {
	b.Table("city")
	b.Key(&Cities.CityID)
	b.BelongsTo(&Cities.Country).OnUpdate(ant.Cascade).OnDelete(ant.Restrict).NoIndex()
	b.Index(&Cities.CountryID).Name("idx_fk_country_id")
}

func (Address) Configure(b *ant.ModelBuilder[Address]) {
	b.Table("address")
	b.Key(&Addresses.AddressID)
	b.BelongsTo(&Addresses.City).OnUpdate(ant.Cascade).OnDelete(ant.Restrict).NoIndex()
	b.Index(&Addresses.CityID).Name("idx_fk_city_id")
}
