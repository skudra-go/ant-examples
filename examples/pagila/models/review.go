package models

import (
	"github.com/skudra-go/ant"
)

// FilmReview is the first table ant created: a customer's review of a film.
type FilmReview struct {
	ant.Base
	FilmID     int32
	CustomerID int32
	Stars      int16
	Body       string

	Film     *Film
	Customer *Customer
}

var FilmReviews = ant.Fields[FilmReview]()

func (FilmReview) Configure(b *ant.ModelBuilder[FilmReview]) {
	b.Table("film_review")
	b.BelongsTo(&FilmReviews.Film).OnDelete(ant.Cascade)
	b.BelongsTo(&FilmReviews.Customer).OnDelete(ant.Cascade)
	b.Check("film_review_stars_check", "stars BETWEEN 1 AND 5")
}
