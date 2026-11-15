package models

import (
	"time"

	"github.com/shopspring/decimal"
	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/dialect/postgres"
)

// Rating is the mpaa_rating enum.
type Rating string

const (
	RatingG    Rating = "G"
	RatingPG   Rating = "PG"
	RatingPG13 Rating = "PG-13"
	RatingR    Rating = "R"
	RatingNC17 Rating = "NC-17"
)

// Language is the language table.
type Language struct {
	LanguageID int32
	Name       string
	UpdatedAt  time.Time `ant:"column=last_update,default=now()"`
}

// Category is the category table.
type Category struct {
	CategoryID int32
	Name       string
	UpdatedAt  time.Time `ant:"column=last_update,default=now()"`
}

// Actor is the actor table.
type Actor struct {
	ActorID   int32
	FirstName string
	LastName  string
	UpdatedAt time.Time `ant:"column=last_update,default=now()"`

	Films []Film
}

// Film is the film table. Fulltext is kept up to date by the film_fulltext_trigger
// trigger, so ant only reads it.
type Film struct {
	FilmID             int32
	Title              string
	Description        *string
	ReleaseYear        *int32
	LanguageID         int32
	OriginalLanguageID *int32
	RentalDuration     int16           `ant:"default=3"`
	RentalRate         decimal.Decimal `ant:"default=4.99"`
	Length             *int16
	ReplacementCost    decimal.Decimal `ant:"default=19.99"`
	Rating             *Rating
	UpdatedAt          time.Time `ant:"column=last_update,default=now()"`
	SpecialFeatures    postgres.Array[string]
	Fulltext           string `ant:"readonly"`
	ImdbID             *string

	Language         *Language
	OriginalLanguage *Language
	Actors           []Actor
	Categories       []Category
	Reviews          []FilmReview
}

// FilmActor is the film_actor link table. It has a last_update column of its own,
// so it is a model as well as the table behind Film.Actors.
type FilmActor struct {
	ActorID   int32
	FilmID    int32
	UpdatedAt time.Time `ant:"column=last_update,default=now()"`

	Actor *Actor
	Film  *Film
}

// FilmCategory is the film_category link table.
type FilmCategory struct {
	FilmID     int32
	CategoryID int32
	UpdatedAt  time.Time `ant:"column=last_update,default=now()"`

	Film     *Film
	Category *Category
}

var (
	Languages      = ant.Fields[Language]()
	Categories     = ant.Fields[Category]()
	Actors         = ant.Fields[Actor]()
	Films          = ant.Fields[Film]()
	FilmActors     = ant.Fields[FilmActor]()
	FilmCategories = ant.Fields[FilmCategory]()
)

func (Language) Configure(b *ant.ModelBuilder[Language]) {
	b.Table("language")
	b.Key(&Languages.LanguageID)
	b.Column(&Languages.Name).Fixed(20)
}

func (Category) Configure(b *ant.ModelBuilder[Category]) {
	b.Table("category")
	b.Key(&Categories.CategoryID)
}

func (Actor) Configure(b *ant.ModelBuilder[Actor]) {
	b.Table("actor")
	b.Key(&Actors.ActorID)
	b.Index(&Actors.LastName).Name("idx_actor_last_name")
	b.ManyToMany(&Actors.Films).Through("film_actor")
}

func (Film) Configure(b *ant.ModelBuilder[Film]) {
	b.Table("film")
	b.Key(&Films.FilmID)
	b.Column(&Films.ReleaseYear).Type("year")
	b.Column(&Films.RentalRate).Precision(4, 2)
	b.Column(&Films.ReplacementCost).Precision(5, 2)

	b.Column(&Films.Rating).
		Enum("mpaa_rating", "G", "PG", "PG-13", "R", "NC-17").
		Default("'G'::mpaa_rating")

	b.Column(&Films.Fulltext).Type("tsvector")
	b.BelongsTo(&Films.Language).OnUpdate(ant.Cascade).OnDelete(ant.Restrict).NoIndex()

	b.BelongsTo(&Films.OriginalLanguage).
		ForeignKey(&Films.OriginalLanguageID).
		References(&Languages.LanguageID).
		OnUpdate(ant.Cascade).
		OnDelete(ant.Restrict).
		NoIndex()

	b.ManyToMany(&Films.Actors).Through("film_actor")
	b.ManyToMany(&Films.Categories).Through("film_category")
	b.Index(&Films.Title).Name("idx_title")
	b.Index(&Films.ImdbID).Unique().Where("imdb_id IS NOT NULL")
	b.Index(&Films.LanguageID).Name("idx_fk_language_id")
	b.Index(&Films.OriginalLanguageID).Name("idx_fk_original_language_id")
	b.Index(&Films.Fulltext).Using("gist").Name("film_fulltext_idx")
}

func (FilmActor) Configure(b *ant.ModelBuilder[FilmActor]) {
	b.Table("film_actor")
	b.CompositeKey(ant.F(&FilmActors.ActorID), ant.F(&FilmActors.FilmID))
	b.BelongsTo(&FilmActors.Actor).OnUpdate(ant.Cascade).OnDelete(ant.Restrict)
	b.BelongsTo(&FilmActors.Film).OnUpdate(ant.Cascade).OnDelete(ant.Restrict).NoIndex()
	b.Index(&FilmActors.FilmID).Name("idx_fk_film_id")
}

func (FilmCategory) Configure(b *ant.ModelBuilder[FilmCategory]) {
	b.Table("film_category")
	b.CompositeKey(ant.F(&FilmCategories.FilmID), ant.F(&FilmCategories.CategoryID))
	b.BelongsTo(&FilmCategories.Film).OnUpdate(ant.Cascade).OnDelete(ant.Restrict)
	b.BelongsTo(&FilmCategories.Category).OnUpdate(ant.Cascade).OnDelete(ant.Restrict).NoIndex()
}
