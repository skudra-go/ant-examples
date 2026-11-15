package models

import (
	"context"
	"time"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/ids"
)

// Subscriber is a row of subscribers.
type Subscriber struct {
	ID        int32                    `json:"id"`
	UUID      string                   `json:"uuid"`
	Email     string                   `json:"email"     ant:"unique"`
	Name      string                   `json:"name"`
	Attribs   ant.JSON[map[string]any] `json:"attribs"`
	Status    SubscriberStatus         `json:"status"`
	CreatedAt time.Time                `json:"created_at"`
	UpdatedAt time.Time                `json:"updated_at"`

	Subscriptions []SubscriberList `json:"lists,omitempty"`
}

// Subscribers is the probe.
var Subscribers = ant.Fields[Subscriber]()

func (Subscriber) Configure(b *ant.ModelBuilder[Subscriber]) {
	b.Column(&Subscribers.UUID).Type("uuid").Unique()
	b.Column(&Subscribers.Attribs).Default("'{}'::jsonb")

	b.Column(&Subscribers.Status).
		Enum("subscriber_status", subscriberStatuses...).
		Default("'enabled'::subscriber_status")

	b.Column(&Subscribers.CreatedAt).Nullable().Default("now()")
	b.Column(&Subscribers.UpdatedAt).Nullable().Default("now()")

	b.IndexExpr("lower(email)").Unique().Name("idx_subs_email")
	b.Index(&Subscribers.Status).Name("idx_subs_status")
	b.Index(&Subscribers.ID).And(&Subscribers.Status).Name("idx_subs_id_status")
	b.Index(&Subscribers.CreatedAt).Name("idx_subs_created_at")
	b.Index(&Subscribers.UpdatedAt).Name("idx_subs_updated_at")
}

// BeforeInsert fills the UUID, as listmonk does in Go before its INSERT.
func (s *Subscriber) BeforeInsert(context.Context) error {
	if s.UUID == "" {
		s.UUID = ids.UUID()
	}

	if s.Status == "" {
		s.Status = SubscriberEnabled
	}

	if s.Attribs.V == nil {
		s.Attribs.V = map[string]any{}
	}

	return nil
}

// SubscriberList is a row of subscriber_lists: a subscription, with its own
// status and meta. Its key is (subscriber_id, list_id).
type SubscriberList struct {
	SubscriberID int32                    `json:"subscriber_id"`
	ListID       int32                    `json:"list_id"`
	Meta         ant.JSON[map[string]any] `json:"meta"`
	Status       SubscriptionStatus       `json:"subscription_status"`
	CreatedAt    time.Time                `json:"created_at"`
	UpdatedAt    time.Time                `json:"updated_at"`

	Subscriber *Subscriber `json:"-"`
	List       *List       `json:"list,omitempty"`
}

// SubscriberLists is the probe.
var SubscriberLists = ant.Fields[SubscriberList]()

func (SubscriberList) Configure(b *ant.ModelBuilder[SubscriberList]) {
	b.CompositeKey(ant.F(&SubscriberLists.SubscriberID), ant.F(&SubscriberLists.ListID))
	b.Column(&SubscriberLists.Meta).Default("'{}'::jsonb")

	b.Column(&SubscriberLists.Status).
		Enum("subscription_status", subscriptionStatuses...).
		Default("'unconfirmed'::subscription_status")

	b.Column(&SubscriberLists.CreatedAt).Nullable().Default("now()")
	b.Column(&SubscriberLists.UpdatedAt).Nullable().Default("now()")

	b.BelongsTo(&SubscriberLists.Subscriber).OnDelete(ant.Cascade).OnUpdate(ant.Cascade).NoIndex()
	b.BelongsTo(&SubscriberLists.List).OnDelete(ant.Cascade).OnUpdate(ant.Cascade).NoIndex()

	b.Index(&SubscriberLists.SubscriberID).Name("idx_sub_lists_sub_id")
	b.Index(&SubscriberLists.ListID).Name("idx_sub_lists_list_id")
	b.Index(&SubscriberLists.Status).Name("idx_sub_lists_status")
}
