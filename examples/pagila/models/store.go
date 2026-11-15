package models

import (
	"time"

	"github.com/shopspring/decimal"
	"github.com/skudra-go/ant"
)

// Store is the store table. manager_staff_id has a unique index but no foreign
// key in Pagila.
type Store struct {
	StoreID        int32
	ManagerStaffID int32
	AddressID      int32
	UpdatedAt      time.Time `ant:"column=last_update,default=now()"`

	Manager *Staff
	Address *Address
}

// Staff is the staff table.
type Staff struct {
	StaffID   int32
	FirstName string
	LastName  string
	AddressID int32
	Email     *string
	StoreID   int32
	Active    bool `ant:"default=true"`
	Username  string
	Password  *string
	UpdatedAt time.Time `ant:"column=last_update,default=now()"`
	Picture   []byte

	Address *Address
	Store   *Store
}

// Customer is the customer table. It has both activebool and a legacy integer
// active column, as Sakila does.
type Customer struct {
	CustomerID int32
	StoreID    int32
	FirstName  string
	LastName   string
	Email      *string
	AddressID  int32
	Activebool bool       `ant:"default=true"`
	CreateDate time.Time  `ant:"default=CURRENT_DATE"`
	UpdatedAt  *time.Time `ant:"column=last_update,default=now()"`
	Active     *int32

	Store    *Store
	Address  *Address
	Rentals  []Rental
	Payments []Payment
}

// Inventory is the inventory table: one copy of a film in a store.
type Inventory struct {
	InventoryID int32
	FilmID      int32
	StoreID     int32
	UpdatedAt   time.Time `ant:"column=last_update,default=now()"`

	Film  *Film
	Store *Store
}

// Rental is the rental table.
type Rental struct {
	RentalID    int32
	RentalDate  time.Time
	InventoryID int32
	CustomerID  int32
	ReturnDate  *time.Time
	StaffID     int32
	UpdatedAt   time.Time `ant:"column=last_update,default=now()"`

	Inventory *Inventory
	Customer  *Customer
	Staff     *Staff
	Payments  []Payment
}

// Payment is the payment table, partitioned by month of payment_date. Its foreign
// keys and indexes live on the partitions, not on the parent.
type Payment struct {
	PaymentID   int32
	CustomerID  int32
	StaffID     int32
	RentalID    int32
	Amount      decimal.Decimal
	PaymentDate time.Time

	Customer *Customer
	Staff    *Staff
	Rental   *Rental
}

var (
	Stores      = ant.Fields[Store]()
	StaffFields = ant.Fields[Staff]()
	Customers   = ant.Fields[Customer]()
	Inventories = ant.Fields[Inventory]()
	Rentals     = ant.Fields[Rental]()
	Payments    = ant.Fields[Payment]()
)

func (Store) Configure(b *ant.ModelBuilder[Store]) {
	b.Table("store")
	b.Key(&Stores.StoreID)

	b.BelongsTo(&Stores.Manager).
		ForeignKey(&Stores.ManagerStaffID).
		References(&StaffFields.StaffID).
		NoForeignKey().
		NoIndex()

	b.BelongsTo(&Stores.Address).OnUpdate(ant.Cascade).OnDelete(ant.Restrict).NoIndex()
	b.Index(&Stores.ManagerStaffID).Unique().Name("idx_unq_manager_staff_id")
}

func (Staff) Configure(b *ant.ModelBuilder[Staff]) {
	b.Table("staff")
	b.Key(&StaffFields.StaffID)
	b.BelongsTo(&StaffFields.Address).OnUpdate(ant.Cascade).OnDelete(ant.Restrict).NoIndex()
	b.BelongsTo(&StaffFields.Store).NoIndex()
}

func (Customer) Configure(b *ant.ModelBuilder[Customer]) {
	b.Table("customer")
	b.Key(&Customers.CustomerID)
	b.Column(&Customers.CreateDate).Type("date")
	b.BelongsTo(&Customers.Store).OnUpdate(ant.Cascade).OnDelete(ant.Restrict).NoIndex()
	b.BelongsTo(&Customers.Address).OnUpdate(ant.Cascade).OnDelete(ant.Restrict).NoIndex()
	b.Index(&Customers.AddressID).Name("idx_fk_address_id")
	b.Index(&Customers.StoreID).Name("idx_fk_store_id")
	b.Index(&Customers.LastName).Name("idx_last_name")
}

func (Inventory) Configure(b *ant.ModelBuilder[Inventory]) {
	b.Table("inventory")
	b.Key(&Inventories.InventoryID)
	b.BelongsTo(&Inventories.Film).OnUpdate(ant.Cascade).OnDelete(ant.Restrict).NoIndex()
	b.BelongsTo(&Inventories.Store).OnUpdate(ant.Cascade).OnDelete(ant.Restrict).NoIndex()
	b.Index(&Inventories.StoreID).And(&Inventories.FilmID).Name("idx_store_id_film_id")
}

func (Rental) Configure(b *ant.ModelBuilder[Rental]) {
	b.Table("rental")
	b.Key(&Rentals.RentalID)
	b.BelongsTo(&Rentals.Inventory).OnUpdate(ant.Cascade).OnDelete(ant.Restrict).NoIndex()
	b.BelongsTo(&Rentals.Customer).OnUpdate(ant.Cascade).OnDelete(ant.Restrict).NoIndex()
	b.BelongsTo(&Rentals.Staff).OnUpdate(ant.Cascade).OnDelete(ant.Restrict).NoIndex()
	b.Index(&Rentals.InventoryID).Name("idx_fk_inventory_id")

	b.Index(&Rentals.RentalDate).
		And(&Rentals.InventoryID).
		And(&Rentals.CustomerID).
		Unique().
		Name("idx_unq_rental_rental_date_inventory_id_customer_id")
}

func (Payment) Configure(b *ant.ModelBuilder[Payment]) {
	b.Table("payment")
	b.CompositeKey(ant.F(&Payments.PaymentDate), ant.F(&Payments.PaymentID))
	b.PartitionByRange(ant.F(&Payments.PaymentDate))
	b.Column(&Payments.PaymentID).Sequence("payment_payment_id_seq")
	b.Column(&Payments.Amount).Precision(5, 2)
	b.BelongsTo(&Payments.Customer).NoForeignKey().NoIndex()
	b.BelongsTo(&Payments.Staff).NoForeignKey().NoIndex()
	b.BelongsTo(&Payments.Rental).NoForeignKey().NoIndex()
}
