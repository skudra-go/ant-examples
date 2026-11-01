package routes

import (
	"net/http"
	"strconv"

	"github.com/skudra-go/ant-examples/blog/routes/requests"
	"github.com/skudra-go/ant-examples/blog/routes/responses"
	"github.com/skudra-go/ant-examples/blog/services"
)

const (
	defaultPageSize = 20
	largestPageSize = 100
)

type postRoutes struct {
	posts    *services.Posts
	comments *services.Comments
}

func number(r *http.Request, name string, fallback int) int {
	if n, err := strconv.Atoi(r.URL.Query().Get(name)); err == nil && n > 0 {
		return n
	}

	return fallback
}

func (p postRoutes) list(w http.ResponseWriter, r *http.Request) {
	size := min(number(r, "size", defaultPageSize), largestPageSize)

	page, err := p.posts.List(r.Context(), r.URL.Query().Get("tag"), number(r, "page", 1), size)
	if err != nil {
		fail(w, err)

		return
	}

	writeJSON(w, http.StatusOK, responses.NewPosts(page))
}

func (p postRoutes) get(w http.ResponseWriter, r *http.Request) {
	post, err := p.posts.Get(r.Context(), r.PathValue("slug"))
	if err != nil {
		fail(w, err)

		return
	}

	writeJSON(w, http.StatusOK, responses.NewPostDetail(post))
}

func (p postRoutes) stats(w http.ResponseWriter, r *http.Request) {
	stats, err := p.posts.Stats(r.Context())
	if err != nil {
		fail(w, err)

		return
	}

	writeJSON(w, http.StatusOK, stats)
}

func (p postRoutes) create(w http.ResponseWriter, r *http.Request) {
	body, ok := read[requests.CreatePost](w, r)
	if !ok {
		return
	}

	post, err := p.posts.Create(r.Context(), services.PostInput{
		AuthorID: body.AuthorID,
		Title:    body.Title,
		Body:     body.Body,
		Tags:     body.Tags,
	})
	if err != nil {
		fail(w, err)

		return
	}

	writeJSON(w, http.StatusCreated, responses.NewPost(post))
}

func (p postRoutes) update(w http.ResponseWriter, r *http.Request) {
	body, ok := read[requests.UpdatePost](w, r)
	if !ok {
		return
	}

	post, err := p.posts.Update(r.Context(), r.PathValue("slug"), services.PostChanges{
		Title: body.Title,
		Body:  body.Body,
		Tags:  body.Tags,
	})
	if err != nil {
		fail(w, err)

		return
	}

	writeJSON(w, http.StatusOK, responses.NewPost(post))
}

func (p postRoutes) publish(w http.ResponseWriter, r *http.Request) {
	if err := p.posts.Publish(r.Context(), r.PathValue("slug")); err != nil {
		fail(w, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (p postRoutes) delete(w http.ResponseWriter, r *http.Request) {
	if err := p.posts.Delete(r.Context(), r.PathValue("slug")); err != nil {
		fail(w, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (p postRoutes) comment(w http.ResponseWriter, r *http.Request) {
	body, ok := read[requests.CreateComment](w, r)
	if !ok {
		return
	}

	comment, err := p.comments.Add(r.Context(), r.PathValue("slug"), body.AuthorName, body.Body)
	if err != nil {
		fail(w, err)

		return
	}

	writeJSON(w, http.StatusCreated, comment)
}
