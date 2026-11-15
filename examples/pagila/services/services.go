// Package services runs the classic Sakila questions against Pagila through ant.
package services

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/dialect/postgres"

	"github.com/skudra-go/ant-examples/examples/pagila/models"
)

// FilmsInCategory returns the films of a category by title, with their actors.
func FilmsInCategory(
	ctx context.Context,
	db *models.AppDB,
	category string,
) ([]*models.Film, error) {
	return db.Films.
		Where(ant.Has(&models.Films.Categories, ant.Eq(&models.Categories.Name, category))).
		Include(&models.Films.Actors, func(q *ant.Query[models.Actor]) *ant.Query[models.Actor] {
			return q.OrderBy(&models.Actors.LastName).ThenBy(&models.Actors.FirstName)
		}).
		OrderBy(&models.Films.Title).
		NoTracking().
		All(ctx)
}

// CustomerTotal is one customer's spending.
type CustomerTotal struct {
	CustomerID int32
	FirstName  string
	LastName   string
	Rentals    int64
	Total      decimal.Decimal
}

var CustomerTotals = ant.Fields[CustomerTotal]()

// TopCustomers ranks customers by what they paid.
func TopCustomers(ctx context.Context, db *models.AppDB, n int) ([]CustomerTotal, error) {
	total := ant.SQL[decimal.Decimal]("sum(?)", ant.Col(&models.Payments.Amount))

	grouped := ant.Then(
		ant.Then(
			db.Payments.Join(&models.Payments.Customer).GroupBy(&models.Customers.CustomerID),
			&models.Customers.FirstName,
		),
		&models.Customers.LastName,
	)

	return ant.Project[CustomerTotal](
		grouped.Order(total.Desc(), ant.Col(&models.Customers.CustomerID).Asc()).Limit(n),
		ant.Into(&CustomerTotals.CustomerID, &models.Customers.CustomerID),
		ant.Into(&CustomerTotals.FirstName, &models.Customers.FirstName),
		ant.Into(&CustomerTotals.LastName, &models.Customers.LastName),
		ant.IntoExpr(&CustomerTotals.Rentals, ant.Count()),
		ant.IntoExpr(&CustomerTotals.Total, total),
	).All(ctx)
}

// StoreMonth is one store's takings in one month.
type StoreMonth struct {
	StoreID  int32
	Month    time.Time
	Payments int64
	Revenue  decimal.Decimal
}

var StoreMonths = ant.Fields[StoreMonth]()

// RevenueByStoreMonth sums the partitioned payment table by the store of the
// staff member who took the payment, and by month in UTC.
func RevenueByStoreMonth(ctx context.Context, db *models.AppDB) ([]StoreMonth, error) {
	month := ant.SQL[time.Time](
		"date_trunc('month', ?, 'UTC')",
		ant.Col(&models.Payments.PaymentDate),
	)

	revenue := ant.SQL[decimal.Decimal]("sum(?)", ant.Col(&models.Payments.Amount))

	grouped := ant.Then(
		db.Payments.Join(&models.Payments.Staff).GroupByExpr(month),
		&models.StaffFields.StoreID,
	)

	return ant.Project[StoreMonth](
		grouped.Order(ant.Col(&models.StaffFields.StoreID).Asc(), month.Asc()),
		ant.Into(&StoreMonths.StoreID, &models.StaffFields.StoreID),
		ant.IntoExpr(&StoreMonths.Month, month),
		ant.IntoExpr(&StoreMonths.Payments, ant.Count()),
		ant.IntoExpr(&StoreMonths.Revenue, revenue),
	).All(ctx)
}

// Overdue is a rental that should have come back by now.
type Overdue struct {
	RentalID   int32
	CustomerID int32
	Title      string
	RentalDate time.Time
	DueDate    time.Time
}

// OverdueRentals lists rentals not returned within the film's rental duration.
func OverdueRentals(ctx context.Context, db *models.AppDB, asOf time.Time) ([]Overdue, error) {
	due := ant.SQL[time.Time](
		"? + make_interval(days => ?)",
		ant.Col(&models.Rentals.RentalDate),
		ant.Col(&models.Films.RentalDuration),
	)

	return db.Rentals.
		Join(&models.Rentals.Inventory).
		Join(&models.Inventories.Film).
		Where(
			ant.IsNull(&models.Rentals.ReturnDate),
			due.Lt(asOf),
		).
		Order(due.Asc(), ant.Col(&models.Rentals.RentalID).Asc()).
		Select(func(r *models.Rental, _ *models.Inventory, f *models.Film) Overdue {
			return Overdue{
				RentalID:   r.RentalID,
				CustomerID: r.CustomerID,
				Title:      f.Title,
				RentalDate: r.RentalDate,
				DueDate:    r.RentalDate.AddDate(0, 0, int(f.RentalDuration)),
			}
		}).
		All(ctx)
}

// CopiesInStock returns the copies of a film a store has on the shelf, using
// Pagila's inventory_in_stock function.
func CopiesInStock(
	ctx context.Context,
	db *models.AppDB,
	filmID, storeID int32,
) ([]*models.Inventory, error) {
	return db.Inventories.
		Where(
			ant.Eq(&models.Inventories.FilmID, filmID),
			ant.Eq(&models.Inventories.StoreID, storeID),
			ant.Func[bool]("inventory_in_stock", ant.Col(&models.Inventories.InventoryID)).Eq(true),
		).
		OrderBy(&models.Inventories.InventoryID).
		NoTracking().
		All(ctx)
}

// FilmInStock calls Pagila's film_in_stock set-returning function.
func FilmInStock(ctx context.Context, db *models.AppDB, filmID, storeID int32) ([]int32, error) {
	return ant.Raw[int32](
		db,
		`SELECT * FROM film_in_stock(?, ?) ORDER BY 1`,
		filmID,
		storeID,
	).All(ctx)
}

// SearchFilms runs a full-text search on the fulltext column the trigger keeps.
func SearchFilms(ctx context.Context, db *models.AppDB, query string) ([]*models.Film, error) {
	return db.Films.
		Where(
			ant.SQLCond(
				"? @@ websearch_to_tsquery('english', ?)",
				ant.Col(&models.Films.Fulltext),
				query,
			),
		).
		OrderBy(&models.Films.Title).
		NoTracking().
		All(ctx)
}

// SearchFilmsMatch is the same search through postgres.Match, which is what the
// docs offer for full-text search.
func SearchFilmsMatch(ctx context.Context, db *models.AppDB, query string) ([]*models.Film, error) {
	return db.Films.
		Where(postgres.Match(&models.Films.Fulltext, "english", query)).
		OrderBy(&models.Films.Title).
		NoTracking().
		All(ctx)
}

// FilmsByRatingAndFeature filters on the enum and on the array column.
func FilmsByRatingAndFeature(
	ctx context.Context,
	db *models.AppDB,
	rating models.Rating,
	feature string,
) ([]*models.Film, error) {
	return db.Films.
		Where(
			ant.Eq(&models.Films.Rating, &rating),
			ant.SQLCond("? = ANY(?)", feature, ant.Col(&models.Films.SpecialFeatures)),
		).
		OrderBy(&models.Films.Title).
		NoTracking().
		All(ctx)
}

// CustomerWithAddress loads a customer with address, city and country.
func CustomerWithAddress(
	ctx context.Context,
	db *models.AppDB,
	id int32,
) (*models.Customer, error) {
	return db.Customers.
		IncludeOne(
			&models.Customers.Address,
			func(q *ant.Query[models.Address]) *ant.Query[models.Address] {
				return q.IncludeOne(
					&models.Addresses.City,
					func(q *ant.Query[models.City]) *ant.Query[models.City] {
						return q.IncludeOne(&models.Cities.Country)
					},
				)
			},
		).
		NoTracking().
		Find(ctx, id)
}

// FilmPage returns one page of films by title.
func FilmPage(
	ctx context.Context,
	db *models.AppDB,
	page, size int,
) (*ant.Page[models.Film], error) {
	return db.Films.OrderBy(&models.Films.Title).
		ThenBy(&models.Films.FilmID).
		NoTracking().
		Page(ctx, page, size)
}

// ReturnRental marks a rental returned through change tracking.
//
// The last_updated trigger overwrites the last_update ant writes, and ant reads
// nothing back after an UPDATE, so the tracked copy is reloaded to show what the
// row really holds.
func ReturnRental(
	ctx context.Context,
	db *models.AppDB,
	id int32,
	at time.Time,
) (*models.Rental, error) {
	session := ant.Session(db, ant.WithTracking(true))

	rental, err := session.Rentals.Find(ctx, id)
	if err != nil {
		return nil, err
	}

	rental.ReturnDate = &at

	if err := session.SaveChanges(ctx); err != nil {
		return nil, err
	}

	if err := session.Rentals.Reload(ctx, rental); err != nil {
		return nil, err
	}

	return rental, nil
}

// TakePayment records a payment in the partitioned payment table.
func TakePayment(ctx context.Context, db *models.AppDB, p *models.Payment) error {
	return db.Payments.Insert(ctx, p)
}
