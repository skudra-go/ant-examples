package services_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/shopspring/decimal"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/anttest"
	"github.com/skudra-go/ant/dialect/postgres"

	"github.com/skudra-go/ant-examples/examples/pagila/models"
	"github.com/skudra-go/ant-examples/examples/pagila/services"
)

var (
	shared *models.AppDB
	plain  *sql.DB
)

func TestMain(m *testing.M) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		os.Exit(0)
	}

	var err error

	shared, err = ant.Open[models.AppDB](
		context.Background(),
		postgres.Dialect,
		dsn,
		ant.WithTracking(false),
	)
	if err != nil {
		panic(err)
	}

	plain, err = sql.Open("pgx", dsn)
	if err != nil {
		panic(err)
	}

	code := m.Run()

	shared.Close()
	plain.Close()
	os.Exit(code)
}

// column reads one column of plain SQL, the oracle every test compares with.
func column[V any](t *testing.T, query string, args ...any) []V {
	t.Helper()

	rows, err := plain.QueryContext(t.Context(), query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var out []V

	for rows.Next() {
		var v V
		if err := rows.Scan(&v); err != nil {
			t.Fatal(err)
		}

		out = append(out, v)
	}

	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	return out
}

func TestFilmsInCategory(t *testing.T) {
	films, err := services.FilmsInCategory(t.Context(), shared, "Documentary")
	if err != nil {
		t.Fatal(err)
	}

	want := column[string](t, `
		SELECT f.title FROM film f
		JOIN film_category fc USING (film_id)
		JOIN category c USING (category_id)
		WHERE c.name = 'Documentary' ORDER BY f.title`)

	var got []string

	for _, f := range films {
		got = append(got, f.Title)
	}

	if !slices.Equal(got, want) {
		t.Fatalf("titles: got %d, want %d", len(got), len(want))
	}

	actors := column[string](
		t,
		`
		SELECT a.last_name FROM film_actor fa JOIN actor a USING (actor_id)
		WHERE fa.film_id = $1 ORDER BY a.last_name, a.first_name`,
		films[0].FilmID,
	)

	var names []string

	for _, a := range films[0].Actors {
		names = append(names, a.LastName)
	}

	if !slices.Equal(names, actors) {
		t.Fatalf("actors of %s: got %v, want %v", films[0].Title, names, actors)
	}
}

func TestTopCustomers(t *testing.T) {
	top, err := services.TopCustomers(t.Context(), shared, 5)
	if err != nil {
		t.Fatal(err)
	}

	want := column[string](t, `
		SELECT customer_id || ':' || count(*) || ':' || sum(amount) FROM payment
		GROUP BY customer_id ORDER BY sum(amount) DESC, customer_id LIMIT 5`)

	var got []string

	for _, c := range top {
		got = append(got, decimal.NewFromInt32(c.CustomerID).String()+":"+
			decimal.NewFromInt(c.Rentals).String()+":"+c.Total.StringFixed(2))
	}

	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestRevenueByStoreMonth(t *testing.T) {
	rows, err := services.RevenueByStoreMonth(t.Context(), shared)
	if err != nil {
		t.Fatal(err)
	}

	want := column[string](t, `
		SELECT s.store_id || ' ' || to_char(date_trunc('month', p.payment_date, 'UTC'), 'YYYY-MM') || ' ' || sum(p.amount)
		FROM payment p JOIN staff s USING (staff_id)
		GROUP BY s.store_id, date_trunc('month', p.payment_date, 'UTC') ORDER BY 1`)

	var got []string

	for _, r := range rows {
		got = append(got, decimal.NewFromInt32(r.StoreID).String()+" "+
			r.Month.UTC().Format("2006-01")+" "+r.Revenue.StringFixed(2))
	}

	slices.Sort(got)

	if !slices.Equal(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

func TestOverdueRentals(t *testing.T) {
	asOf := time.Date(2022, 9, 1, 0, 0, 0, 0, time.UTC)

	overdue, err := services.OverdueRentals(t.Context(), shared, asOf)
	if err != nil {
		t.Fatal(err)
	}

	want := column[int32](
		t,
		`
		SELECT r.rental_id FROM rental r
		JOIN inventory i USING (inventory_id) JOIN film f USING (film_id)
		WHERE r.return_date IS NULL AND r.rental_date + make_interval(days => f.rental_duration) < $1
		ORDER BY r.rental_date + make_interval(days => f.rental_duration), r.rental_id`,
		asOf,
	)

	var got []int32

	for _, o := range overdue {
		got = append(got, o.RentalID)
	}

	if !slices.Equal(got, want) {
		t.Fatalf("got %d rentals, want %d", len(got), len(want))
	}
}

func TestCopiesInStock(t *testing.T) {
	copies, err := services.CopiesInStock(t.Context(), shared, 1, 1)
	if err != nil {
		t.Fatal(err)
	}

	ids, err := services.FilmInStock(t.Context(), shared, 1, 1)
	if err != nil {
		t.Fatal(err)
	}

	want := column[int32](t, `SELECT * FROM film_in_stock(1, 1) ORDER BY 1`)

	var got []int32

	for _, c := range copies {
		got = append(got, c.InventoryID)
	}

	if !slices.Equal(got, want) || !slices.Equal(ids, want) {
		t.Fatalf("got %v and %v, want %v", got, ids, want)
	}
}

func TestSearchFilms(t *testing.T) {
	films, err := services.SearchFilms(t.Context(), shared, "astronaut boat")
	if err != nil {
		t.Fatal(err)
	}

	want := column[string](t, `
		SELECT title FROM film WHERE fulltext @@ websearch_to_tsquery('english', 'astronaut boat')
		ORDER BY title`)

	var got []string

	for _, f := range films {
		got = append(got, f.Title)
	}

	if len(want) == 0 || !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestSearchFilmsMatch records that postgres.Match cannot search a tsvector
// column: it wraps the column in to_tsvector, which takes text.
func TestSearchFilmsMatch(t *testing.T) {
	_, err := services.SearchFilmsMatch(t.Context(), shared, "astronaut boat")
	if ant.SQLState(err) != "42883" {
		t.Fatalf("postgres.Match on a tsvector column: %v; update the README", err)
	}
}

func TestFilmsByRatingAndFeature(t *testing.T) {
	films, err := services.FilmsByRatingAndFeature(
		t.Context(),
		shared,
		models.RatingPG13,
		"Trailers",
	)
	if err != nil {
		t.Fatal(err)
	}

	want := column[string](t, `
		SELECT title FROM film WHERE rating = 'PG-13' AND 'Trailers' = ANY(special_features)
		ORDER BY title`)

	var got []string

	for _, f := range films {
		got = append(got, f.Title)
	}

	if len(want) == 0 || !slices.Equal(got, want) {
		t.Fatalf("got %d, want %d", len(got), len(want))
	}

	if !slices.Contains(films[0].SpecialFeatures, "Trailers") ||
		*films[0].Rating != models.RatingPG13 {
		t.Fatalf("scanned %v %v", films[0].SpecialFeatures, *films[0].Rating)
	}
}

func TestCustomerWithAddress(t *testing.T) {
	c, err := services.CustomerWithAddress(t.Context(), shared, 1)
	if err != nil {
		t.Fatal(err)
	}

	want := column[string](t, `
		SELECT cu.first_name || ' ' || a.address || ' ' || ci.city || ' ' || co.country
		FROM customer cu JOIN address a USING (address_id) JOIN city ci USING (city_id)
		JOIN country co USING (country_id) WHERE cu.customer_id = 1`)

	got := c.FirstName + " " + c.Address.Address + " " + c.Address.City.City + " " + c.Address.City.Country.Country
	if got != want[0] {
		t.Fatalf("got %q, want %q", got, want[0])
	}
}

func TestFilmPage(t *testing.T) {
	page, err := services.FilmPage(t.Context(), shared, 3, 20)
	if err != nil {
		t.Fatal(err)
	}

	want := column[string](t, `SELECT title FROM film ORDER BY title, film_id LIMIT 20 OFFSET 40`)

	var got []string

	for _, f := range page.Items {
		got = append(got, f.Title)
	}

	if !slices.Equal(got, want) || page.Pages() != 50 || !page.HasNext() {
		t.Fatalf("got %v pages=%d", got, page.Pages())
	}
}

func TestViews(t *testing.T) {
	ctx := t.Context()

	rows, err := shared.CustomerList.
		Where(ant.Eq(&models.CustomerList.Country, "Poland")).
		OrderBy(&models.CustomerList.ID).
		All(ctx)
	if err != nil {
		t.Fatal(err)
	}

	want := column[string](
		t,
		`SELECT "zip code" FROM customer_list WHERE country = 'Poland' ORDER BY id`,
	)

	if len(rows) != len(want) || rows[0].ZipCode == nil || *rows[0].ZipCode != want[0] {
		t.Fatalf("got %d rows, want %d", len(rows), len(want))
	}

	sales, err := shared.SalesByStore.OrderBy(&models.SalesByStore.Store).All(ctx)
	if err != nil {
		t.Fatal(err)
	}

	total := column[string](t, `SELECT total_sales::text FROM sales_by_store ORDER BY store`)
	if len(sales) != len(total) || sales[0].TotalSales.StringFixed(2) != total[0] {
		t.Fatalf("got %v, want %v", sales, total)
	}

	err = shared.SalesByStore.Insert(ctx, &models.SalesByStoreRow{Store: "x"})
	if !errors.Is(err, ant.ErrReadOnly) {
		t.Fatalf("insert into a view: %v", err)
	}
}

func TestRewardsReport(t *testing.T) {
	// rewards_report returns SETOF customer, so it scans straight into the model.
	// It looks at last month, and Pagila's payments end in 2022, so it is empty.
	got, err := ant.FromFunc[models.Customer](shared, "rewards_report(?, ?)", 7, 20.0).
		OrderBy(&models.Customers.CustomerID).
		All(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	want := column[int32](t, `SELECT customer_id FROM rewards_report(7, 20.0) ORDER BY 1`)
	if len(got) != len(want) {
		t.Fatalf("got %d, want %d", len(got), len(want))
	}
}

func TestReturnRental(t *testing.T) {
	db := anttest.Isolate(t, shared)
	ctx := t.Context()

	open := column[int32](t, `SELECT rental_id FROM rental WHERE return_date IS NULL ORDER BY rental_id LIMIT 1`)[0]
	at := time.Date(2022, 9, 1, 12, 0, 0, 0, time.UTC)

	rental, err := services.ReturnRental(ctx, db, open, at)
	if err != nil {
		t.Fatal(err)
	}

	stored, err := ant.Raw[time.Time](
		db,
		`SELECT last_update FROM rental WHERE rental_id = ?`,
		open,
	).First(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if !rental.UpdatedAt.Equal(*stored) || !rental.ReturnDate.Equal(at) {
		t.Fatalf("tracked last_update %v, stored %v", rental.UpdatedAt, *stored)
	}
}

// TestTriggerColumnsGoStale shows why ReturnRental reloads: after SaveChanges the
// tracked film keeps ant's last_update and the old fulltext, which the triggers
// replaced in the row.
func TestTriggerColumnsGoStale(t *testing.T) {
	db := anttest.Isolate(t, shared)
	ctx := t.Context()
	session := ant.Session(db, ant.WithTracking(true))

	film, err := session.Films.Find(ctx, int32(1))
	if err != nil {
		t.Fatal(err)
	}

	film.Title = "ZEBRA QUEST"

	if err := session.SaveChanges(ctx); err != nil {
		t.Fatal(err)
	}

	stored, err := ant.Raw[string](
		db,
		`SELECT fulltext::text FROM film WHERE film_id = 1`,
	).First(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if film.Fulltext == *stored {
		t.Fatal(
			"the tracked fulltext is fresh: ant now reads trigger columns back, update the README",
		)
	}

	if err := session.Films.Reload(ctx, film); err != nil {
		t.Fatal(err)
	}

	if film.Fulltext != *stored || !strings.Contains(film.Fulltext, "'zebra'") {
		t.Fatalf("after Reload: %q, stored %q", film.Fulltext, *stored)
	}
}

func TestTakePayment(t *testing.T) {
	db := anttest.Isolate(t, shared)
	ctx := t.Context()

	p := &models.Payment{
		CustomerID:  1,
		StaffID:     1,
		RentalID:    76,
		Amount:      decimal.RequireFromString("2.99"),
		PaymentDate: time.Date(2022, 7, 30, 10, 0, 0, 0, time.UTC),
	}

	if err := services.TakePayment(ctx, db, p); err != nil {
		t.Fatal(err)
	}

	if p.PaymentID == 0 {
		t.Fatal("payment_id was not read back")
	}

	found, err := db.Payments.FindKey(ctx, p.PaymentDate, p.PaymentID)
	if err != nil {
		t.Fatal(err)
	}

	if !found.Amount.Equal(p.Amount) {
		t.Fatalf("amount %v", found.Amount)
	}

	late := &models.Payment{
		CustomerID:  1,
		StaffID:     1,
		RentalID:    76,
		Amount:      decimal.RequireFromString("1.00"),
		PaymentDate: time.Date(2023, 1, 5, 10, 0, 0, 0, time.UTC),
	}

	err = db.Tx(ctx, func(tx *models.AppDB) error {
		return tx.Payments.Insert(ctx, late)
	})
	if ant.SQLState(err) != "23514" {
		t.Fatalf("payment outside every partition: %v", err)
	}
}

func TestInsertFilm(t *testing.T) {
	db := anttest.Isolate(t, shared)
	ctx := t.Context()

	rating := models.RatingPG13
	year := int32(2026)
	film := &models.Film{
		Title:           "ANT ADVENTURE",
		Description:     new("A typed ORM meets a brownfield database"),
		ReleaseYear:     &year,
		LanguageID:      1,
		Rating:          &rating,
		SpecialFeatures: postgres.Array[string]{"Trailers", "Behind the Scenes"},
	}

	if err := db.Films.Insert(ctx, film); err != nil {
		t.Fatal(err)
	}

	stored, err := ant.Raw[string](
		db,
		`SELECT fulltext::text FROM film WHERE film_id = ?`,
		film.FilmID,
	).First(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if film.Fulltext != *stored || !strings.Contains(film.Fulltext, "'ant'") {
		t.Fatalf("fulltext: struct %q, stored %q", film.Fulltext, *stored)
	}

	if film.RentalRate.StringFixed(2) != "4.99" || film.RentalDuration != 3 {
		t.Fatalf("defaults not read back: %v %v", film.RentalRate, film.RentalDuration)
	}
}

func TestDeleteRestricted(t *testing.T) {
	db := anttest.Isolate(t, shared)

	_, err := db.Languages.Where(ant.Eq(&models.Languages.LanguageID, 1)).Delete(t.Context())
	if !ant.IsForeignKeyViolation(err) {
		t.Fatalf("got %v", err)
	}
}
