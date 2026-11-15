package models

// listmonk keeps its states in PostgreSQL enum types. Each is a named string
// type here, and the column is declared with .Enum in Configure.

type SubscriberStatus string

const (
	SubscriberEnabled     SubscriberStatus = "enabled"
	SubscriberDisabled    SubscriberStatus = "disabled"
	SubscriberBlocklisted SubscriberStatus = "blocklisted"
)

var subscriberStatuses = []string{"enabled", "disabled", "blocklisted"}

type SubscriptionStatus string

const (
	SubscriptionUnconfirmed  SubscriptionStatus = "unconfirmed"
	SubscriptionConfirmed    SubscriptionStatus = "confirmed"
	SubscriptionUnsubscribed SubscriptionStatus = "unsubscribed"
)

var subscriptionStatuses = []string{"unconfirmed", "confirmed", "unsubscribed"}

type ListType string

const (
	ListPublic    ListType = "public"
	ListPrivate   ListType = "private"
	ListTemporary ListType = "temporary"
)

type ListOptin string

const (
	OptinSingle ListOptin = "single"
	OptinDouble ListOptin = "double"
)

type CampaignStatus string

const (
	CampaignDraft     CampaignStatus = "draft"
	CampaignRunning   CampaignStatus = "running"
	CampaignScheduled CampaignStatus = "scheduled"
	CampaignPaused    CampaignStatus = "paused"
	CampaignCancelled CampaignStatus = "cancelled"
	CampaignFinished  CampaignStatus = "finished"
)

type CampaignType string

const (
	CampaignRegular CampaignType = "regular"
	CampaignOptin   CampaignType = "optin"
)

type ContentType string

const (
	ContentRichtext ContentType = "richtext"
	ContentHTML     ContentType = "html"
	ContentPlain    ContentType = "plain"
	ContentMarkdown ContentType = "markdown"
	ContentVisual   ContentType = "visual"
)

type BounceType string

const (
	BounceSoft      BounceType = "soft"
	BounceHard      BounceType = "hard"
	BounceComplaint BounceType = "complaint"
)

type TemplateType string

const (
	TemplateCampaign       TemplateType = "campaign"
	TemplateCampaignVisual TemplateType = "campaign_visual"
	TemplateTx             TemplateType = "tx"
)

type UserType string

type UserStatus string

type RoleType string
