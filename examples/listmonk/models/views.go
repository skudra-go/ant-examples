package models

import (
	"encoding/json"
	"time"

	"github.com/skudra-go/ant"
)

// The dashboards read three materialized views that schema.sql creates. They
// are models with b.View: read-only, and not managed by migrations.

// ListSubscriberStat is a row of mat_list_subscriber_stats. Row list_id = 0
// holds the count of all subscribers.
type ListSubscriberStat struct {
	UpdatedAt       time.Time
	ListID          int32
	Status          *SubscriptionStatus
	SubscriberCount int64
}

// ListStats is the probe.
var ListStats = ant.Fields[ListSubscriberStat]()

func (ListSubscriberStat) Configure(b *ant.ModelBuilder[ListSubscriberStat]) {
	b.View("mat_list_subscriber_stats")
	b.NoKey()
}

// DashboardCounts is the single row of mat_dashboard_counts.
type DashboardCounts struct {
	UpdatedAt time.Time
	Data      ant.JSON[json.RawMessage]
}

// DashboardCountsFields is the probe.
var DashboardCountsFields = ant.Fields[DashboardCounts]()

func (DashboardCounts) Configure(b *ant.ModelBuilder[DashboardCounts]) {
	b.View("mat_dashboard_counts")
	b.NoKey()
}

// DashboardCharts is the single row of mat_dashboard_charts.
type DashboardCharts struct {
	UpdatedAt time.Time
	Data      ant.JSON[json.RawMessage]
}

// DashboardChartsFields is the probe.
var DashboardChartsFields = ant.Fields[DashboardCharts]()

func (DashboardCharts) Configure(b *ant.ModelBuilder[DashboardCharts]) {
	b.View("mat_dashboard_charts")
	b.NoKey()
}
