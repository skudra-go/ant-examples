// Package models describes listmonk's existing PostgreSQL schema as ant models.
// The tables were created by listmonk's own schema.sql; these structs were
// written by hand to match them, and `migrate drift` checks that they do.
package models

import "github.com/skudra-go/ant"

// AppDB is the database.
type AppDB struct {
	ant.Context
	Subscribers     ant.Table[Subscriber]
	Lists           ant.Table[List]
	SubscriberLists ant.Table[SubscriberList]
	Templates       ant.Table[Template]
	Campaigns       ant.Table[Campaign]
	CampaignLists   ant.Table[CampaignList]
	CampaignViews   ant.Table[CampaignView]
	Media           ant.Table[Media]
	CampaignMedia   ant.Table[CampaignMedia]
	Links           ant.Table[Link]
	LinkClicks      ant.Table[LinkClick]
	Bounces         ant.Table[Bounce]
	Settings        ant.Table[Setting]
	Roles           ant.Table[Role]
	Users           ant.Table[User]
	Sessions        ant.Table[Session]
	ListStats       ant.Table[ListSubscriberStat]
	DashboardCounts ant.Table[DashboardCounts]
	DashboardCharts ant.Table[DashboardCharts]
}
