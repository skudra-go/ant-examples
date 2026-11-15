package models

import (
	"github.com/shopspring/decimal"
	"github.com/skudra-go/ant"
)

// CustomerListRow is a row of the customer_list view.
type CustomerListRow struct {
	ID      int32
	Name    string
	Address string
	ZipCode *string `ant:"column=zip code"`
	Phone   string
	City    string
	Country string
	Notes   string
	SID     int32 `ant:"column=sid"`
}

// SalesByStoreRow is a row of the sales_by_store view.
type SalesByStoreRow struct {
	Store      string
	Manager    string
	TotalSales decimal.Decimal
}

var (
	CustomerList = ant.Fields[CustomerListRow]()
	SalesByStore = ant.Fields[SalesByStoreRow]()
)

func (CustomerListRow) Configure(b *ant.ModelBuilder[CustomerListRow]) {
	b.View("customer_list")
	b.NoKey()
}

func (SalesByStoreRow) Configure(b *ant.ModelBuilder[SalesByStoreRow]) {
	b.View("sales_by_store")
}
