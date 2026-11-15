// Command pagila answers the classic Sakila questions on Pagila through ant.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/dialect/postgres"

	"github.com/skudra-go/ant-examples/examples/pagila/models"
	"github.com/skudra-go/ant-examples/examples/pagila/services"
)

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	db, err := ant.Open[models.AppDB](
		ctx,
		postgres.Dialect,
		os.Getenv("DATABASE_URL"),
		ant.WithTracking(false),
	)
	if err != nil {
		return err
	}
	defer db.Close()

	films, err := services.FilmsInCategory(ctx, db, "Documentary")
	if err != nil {
		return err
	}

	fmt.Printf("Documentaries: %d. The first, %s, stars", len(films), films[0].Title)

	for _, a := range films[0].Actors {
		fmt.Printf(" %s %s,", a.FirstName, a.LastName)
	}

	fmt.Println()

	top, err := services.TopCustomers(ctx, db, 3)
	if err != nil {
		return err
	}

	fmt.Println("\nTop customers:")

	for _, c := range top {
		fmt.Printf(
			"  %-10s %-12s %3d payments %8s\n",
			c.FirstName,
			c.LastName,
			c.Rentals,
			c.Total.StringFixed(2),
		)
	}

	months, err := services.RevenueByStoreMonth(ctx, db)
	if err != nil {
		return err
	}

	fmt.Println("\nRevenue by store and month (payment is partitioned by month):")

	for _, m := range months {
		fmt.Printf(
			"  store %d %s %5d payments %10s\n",
			m.StoreID,
			m.Month.UTC().Format("2006-01"),
			m.Payments,
			m.Revenue.StringFixed(2),
		)
	}

	overdue, err := services.OverdueRentals(ctx, db, time.Date(2022, 9, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		return err
	}

	fmt.Printf(
		"\nOverdue on 2022-09-01: %d rentals, the oldest %s, due %s\n",
		len(overdue),
		overdue[0].Title,
		overdue[0].DueDate.UTC().Format(time.DateOnly),
	)

	copies, err := services.CopiesInStock(ctx, db, 1, 1)
	if err != nil {
		return err
	}

	fmt.Printf("Copies of film 1 on the shelf in store 1: %d\n", len(copies))

	found, err := services.SearchFilms(ctx, db, "astronaut boat")
	if err != nil {
		return err
	}

	fmt.Printf(
		"Full-text search for \"astronaut boat\": %d films, first %s\n",
		len(found),
		found[0].Title,
	)

	trailers, err := services.FilmsByRatingAndFeature(ctx, db, models.RatingPG13, "Trailers")
	if err != nil {
		return err
	}

	fmt.Printf("PG-13 films with trailers: %d\n", len(trailers))

	c, err := services.CustomerWithAddress(ctx, db, 1)
	if err != nil {
		return err
	}

	fmt.Printf(
		"Customer 1: %s %s, %s, %s, %s\n",
		c.FirstName,
		c.LastName,
		c.Address.Address,
		c.Address.City.City,
		c.Address.City.Country.Country,
	)

	page, err := services.FilmPage(ctx, db, 3, 5)
	if err != nil {
		return err
	}

	fmt.Printf("Films, page 3 of %d:", page.Pages())

	for _, f := range page.Items {
		fmt.Printf(" %s,", f.Title)
	}

	fmt.Println()

	return nil
}
