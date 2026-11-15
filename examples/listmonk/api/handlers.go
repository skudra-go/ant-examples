package api

import (
	"net/http"

	"github.com/skudra-go/ant-examples/listmonk/core"
	"github.com/skudra-go/ant-examples/listmonk/models"
)

func (s *Server) querySubscribers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	listIDs, err := ids(q["list_id"])
	if err != nil {
		s.fail(w, r, err)

		return
	}

	offset, limit, page := paging(r)

	subs, total, err := s.core().QuerySubscribers(
		r.Context(),
		core.SubscriberQuery{
			Search:    q.Get("search"),
			Expr:      q.Get("query"),
			ListIDs:   listIDs,
			SubStatus: models.SubscriptionStatus(q.Get("subscription_status")),
		},
		q.Get("order_by"),
		q.Get("order"),
		offset,
		limit,
	)
	if err != nil {
		s.fail(w, r, err)

		return
	}

	out := Page[SubscriberDTO]{
		Results: make([]SubscriberDTO, len(subs)),
		Total:   total,
		PerPage: limit,
		Page:    page,
	}

	for i, sub := range subs {
		out.Results[i] = subscriberDTO(sub)
	}

	s.ok(w, out)
}

func (s *Server) getSubscriber(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.fail(w, r, err)

		return
	}

	sub, err := s.core().GetSubscriber(r.Context(), id, "", "")
	if err != nil {
		s.fail(w, r, err)

		return
	}

	s.ok(w, subscriberDTO(sub))
}

type subscriberRequest struct {
	Email                   string                  `json:"email"`
	Name                    string                  `json:"name"`
	Status                  models.SubscriberStatus `json:"status"`
	Lists                   []int32                 `json:"lists"`
	Attribs                 map[string]any          `json:"attribs"`
	PreconfirmSubscriptions bool                    `json:"preconfirm_subscriptions"`
}

func (req subscriberRequest) subscriptionStatus() models.SubscriptionStatus {
	if req.PreconfirmSubscriptions {
		return models.SubscriptionConfirmed
	}

	return models.SubscriptionUnconfirmed
}

func validStatus(s models.SubscriberStatus) bool {
	switch s {
	case "", models.SubscriberEnabled, models.SubscriberDisabled, models.SubscriberBlocklisted:
		return true
	}

	return false
}

func (s *Server) createSubscriber(w http.ResponseWriter, r *http.Request) {
	var req subscriberRequest
	if err := decode(r, &req); err != nil {
		s.fail(w, r, err)

		return
	}

	if req.Email == "" || req.Name == "" || !validStatus(req.Status) {
		s.fail(w, r, core.ErrInvalid)

		return
	}

	c := s.core()
	sub := core.NewSubscriber(req.Email, req.Name, req.Attribs)

	if req.Status != "" {
		sub.Status = req.Status
	}

	if err := c.InsertSubscriber(
		r.Context(),
		sub,
		req.Lists,
		req.subscriptionStatus(),
	); err != nil {
		s.fail(w, r, err)

		return
	}

	// Read back with the lists, on the same session.
	out, err := c.GetSubscriber(r.Context(), sub.ID, "", "")
	if err != nil {
		s.fail(w, r, err)

		return
	}

	s.ok(w, subscriberDTO(out))
}

func (s *Server) updateSubscriber(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.fail(w, r, err)

		return
	}

	var req subscriberRequest
	if err := decode(r, &req); err != nil {
		s.fail(w, r, err)

		return
	}

	if !validStatus(req.Status) {
		s.fail(w, r, core.ErrInvalid)

		return
	}

	sub, err := s.core().UpdateSubscriberWithLists(
		r.Context(),
		id,
		core.SubscriberPatch{
			Email:   req.Email,
			Name:    req.Name,
			Status:  req.Status,
			Attribs: req.Attribs,
		},
		req.Lists,
		req.subscriptionStatus(),
		true,
	)
	if err != nil {
		s.fail(w, r, err)

		return
	}

	s.ok(w, subscriberDTO(sub))
}

func (s *Server) blocklistByQuery(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query   string  `json:"query"`
		ListIDs []int32 `json:"list_ids"`
	}

	if err := decode(r, &req); err != nil {
		s.fail(w, r, err)

		return
	}

	n, err := s.core().
		BlocklistSubscribersByQuery(r.Context(), core.SubscriberQuery{Expr: req.Query, ListIDs: req.ListIDs})
	if err != nil {
		s.fail(w, r, err)

		return
	}

	s.ok(w, map[string]int64{"count": n})
}

func (s *Server) queryLists(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	offset, limit, page := paging(r)

	lists, total, err := s.core().QueryLists(
		r.Context(),
		core.ListQuery{
			Search: q.Get("query"),
			Type:   models.ListType(q.Get("type")),
			Optin:  models.ListOptin(q.Get("optin")),
			Tags:   q["tag"],
		},
		q.Get("order_by"),
		q.Get("order"),
		offset,
		limit,
	)
	if err != nil {
		s.fail(w, r, err)

		return
	}

	s.ok(w, Page[ListDTO]{
		Results: lists,
		Total:   total,
		PerPage: limit,
		Page:    page,
	})
}

type listRequest struct {
	Name        string           `json:"name"`
	Type        models.ListType  `json:"type"`
	Optin       models.ListOptin `json:"optin"`
	Tags        []string         `json:"tags"`
	Description string           `json:"description"`
}

func (s *Server) createList(w http.ResponseWriter, r *http.Request) {
	var req listRequest
	if err := decode(r, &req); err != nil {
		s.fail(w, r, err)

		return
	}

	if req.Name == "" ||
		(req.Type != models.ListPublic && req.Type != models.ListPrivate && req.Type != models.ListTemporary) {
		s.fail(w, r, core.ErrInvalid)

		return
	}

	l := &models.List{
		Name:        req.Name,
		Type:        req.Type,
		Optin:       req.Optin,
		Tags:        req.Tags,
		Description: req.Description,
	}

	if err := s.core().CreateList(r.Context(), l); err != nil {
		s.fail(w, r, err)

		return
	}

	s.ok(w, l)
}

func (s *Server) updateList(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.fail(w, r, err)

		return
	}

	var req listRequest
	if err := decode(r, &req); err != nil {
		s.fail(w, r, err)

		return
	}

	l, err := s.core().UpdateList(r.Context(), id, core.ListPatch(req))
	if err != nil {
		s.fail(w, r, err)

		return
	}

	s.ok(w, l)
}

func (s *Server) campaignStatus(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.fail(w, r, err)

		return
	}

	var req struct {
		Status models.CampaignStatus `json:"status"`
	}

	if err := decode(r, &req); err != nil {
		s.fail(w, r, err)

		return
	}

	switch req.Status {
	case models.CampaignRunning,
		models.CampaignPaused,
		models.CampaignCancelled,
		models.CampaignScheduled:
	default:
		s.fail(w, r, core.ErrInvalid)

		return
	}

	camp, err := s.core().UpdateCampaignStatus(r.Context(), id, req.Status)
	if err != nil {
		s.fail(w, r, err)

		return
	}

	s.ok(w, camp)
}

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.core().GetSettings(r.Context())
	if err != nil {
		s.fail(w, r, err)

		return
	}

	// Passwords never leave the server, as in listmonk.
	for _, key := range []string{"smtp", "bounce.mailboxes"} {
		delete(settings, key)
	}

	s.ok(w, settings)
}

func (s *Server) linkRedirect(w http.ResponseWriter, r *http.Request) {
	url, err := s.core().
		RegisterLinkClick(r.Context(), r.PathValue("link"), r.PathValue("campaign"), r.PathValue("subscriber"))
	if err != nil {
		s.fail(w, r, err)

		return
	}

	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}
