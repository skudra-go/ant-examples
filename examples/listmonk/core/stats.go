package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/skudra-go/ant"

	"github.com/skudra-go/ant-examples/listmonk/models"
)

// DashboardCounts returns the cached dashboard counts, from a materialized view.
func (c *Core) DashboardCounts(ctx context.Context) (json.RawMessage, error) {
	row, err := c.db.DashboardCounts.NoTracking().First(ctx)
	if err != nil {
		return nil, err
	}

	return row.Data.V, nil
}

// RefreshDashboards refreshes the dashboard views. Raw SQL: ant has no
// statement for it.
func (c *Core) RefreshDashboards(ctx context.Context) error {
	for _, view := range []string{"mat_dashboard_counts", "mat_dashboard_charts", "mat_list_subscriber_stats"} {
		if _, err := ant.Raw[any](
			c.db,
			"REFRESH MATERIALIZED VIEW CONCURRENTLY "+view,
		).Exec(ctx); err != nil {
			return err
		}
	}

	return nil
}

// DayCount is a count for one day.
type DayCount struct {
	Day   time.Time `json:"date"`
	Count int64     `json:"count"`
}

var DC = ant.Fields[DayCount]()

// CampaignViewsByDay counts a campaign's views per UTC day, with a row for
// every day between from and to, zeros included. The series of days needs
// generate_series, which ant has no source for, so it is raw SQL.
func (c *Core) CampaignViewsByDay(
	ctx context.Context,
	campID int32,
	from, to time.Time,
) ([]DayCount, error) {
	return ant.Raw[DayCount](
		c.db,
		`SELECT d.day, COUNT(v.id) AS count
		FROM generate_series(@from::date, @to::date, interval '1 day') AS d(day)
		LEFT JOIN campaign_views v
			ON v.campaign_id = @camp AND TIMEZONE('UTC', v.created_at)::date = d.day::date
		GROUP BY d.day
		ORDER BY d.day`,
		sql.Named("from", from.UTC()),
		sql.Named("to", to.UTC()),
		sql.Named("camp", campID),
	).All(ctx)
}

// CampaignViewsByDayTyped is the same count without the empty days, in typed
// ant: grouped by an expression.
func (c *Core) CampaignViewsByDayTyped(
	ctx context.Context,
	campID int32,
	from, to time.Time,
) ([]DayCount, error) {
	V := models.CampaignViews
	day := ant.SQL[time.Time]("TIMEZONE('UTC', ?)::date", ant.Col(&V.CreatedAt))

	return ant.Project[DayCount](
		c.db.CampaignViews.
			Where(
				ant.Eq(&V.CampaignID, campID),
				day.Gte(from.UTC().Truncate(24*time.Hour)),
				day.Lte(to.UTC().Truncate(24*time.Hour)),
			).
			GroupByExpr(day).
			Order(day.Asc()),
		ant.IntoExpr(&DC.Day, day),
		ant.IntoExpr(&DC.Count, ant.Count()),
	).All(ctx)
}
