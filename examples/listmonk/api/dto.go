package api

import (
	"time"

	"github.com/skudra-go/ant-examples/listmonk/core"
	"github.com/skudra-go/ant-examples/listmonk/models"
)

// The API answers with DTOs, not models: the shape is listmonk's, which nests
// a list inside each subscription the other way round from the tables.

// SubscriptionDTO is a list as one subscriber's subscription.
type SubscriptionDTO struct {
	ID                    int32                     `json:"id"`
	UUID                  string                    `json:"uuid"`
	Name                  string                    `json:"name"`
	Type                  models.ListType           `json:"type"`
	Optin                 models.ListOptin          `json:"optin"`
	Tags                  []string                  `json:"tags"`
	SubscriptionStatus    models.SubscriptionStatus `json:"subscription_status"`
	SubscriptionCreatedAt time.Time                 `json:"subscription_created_at"`
	SubscriptionUpdatedAt time.Time                 `json:"subscription_updated_at"`
	Meta                  map[string]any            `json:"meta"`
}

// SubscriberDTO is a subscriber with its lists.
type SubscriberDTO struct {
	ID        int32                   `json:"id"`
	UUID      string                  `json:"uuid"`
	Email     string                  `json:"email"`
	Name      string                  `json:"name"`
	Attribs   map[string]any          `json:"attribs"`
	Status    models.SubscriberStatus `json:"status"`
	Lists     []SubscriptionDTO       `json:"lists"`
	CreatedAt time.Time               `json:"created_at"`
	UpdatedAt time.Time               `json:"updated_at"`
}

func subscriberDTO(s *models.Subscriber) SubscriberDTO {
	out := SubscriberDTO{
		ID:        s.ID,
		UUID:      s.UUID,
		Email:     s.Email,
		Name:      s.Name,
		Attribs:   s.Attribs.V,
		Status:    s.Status,
		Lists:     []SubscriptionDTO{},
		CreatedAt: s.CreatedAt.UTC(),
		UpdatedAt: s.UpdatedAt.UTC(),
	}

	for _, sl := range s.Subscriptions {
		if sl.List == nil {
			continue
		}

		tags := []string(sl.List.Tags)
		if tags == nil {
			tags = []string{}
		}

		out.Lists = append(out.Lists, SubscriptionDTO{
			ID:                    sl.List.ID,
			UUID:                  sl.List.UUID,
			Name:                  sl.List.Name,
			Type:                  sl.List.Type,
			Optin:                 sl.List.Optin,
			Tags:                  tags,
			SubscriptionStatus:    sl.Status,
			SubscriptionCreatedAt: sl.CreatedAt.UTC(),
			SubscriptionUpdatedAt: sl.UpdatedAt.UTC(),
			Meta:                  sl.Meta.V,
		})
	}

	return out
}

// Page is listmonk's paged result.
type Page[T any] struct {
	Results []T   `json:"results"`
	Total   int64 `json:"total"`
	PerPage int   `json:"per_page"`
	Page    int   `json:"page"`
}

// ListDTO is a list with its counts.
type ListDTO = core.ListWithCounts
