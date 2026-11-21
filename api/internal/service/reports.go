package service

import (
	"context"
	"time"

	"ticketing/api/internal/apperr"
	"ticketing/api/internal/domain"
)

const (
	recentLimit     = 5
	maxExportRows   = 10000
	maxTrendBuckets = 400
)

// ParseReportRange validates optional RFC 3339 from/to values and an IANA timezone.
func ParseReportRange(from, to, tz string) (domain.ReportRange, error) {
	rng := domain.ReportRange{Location: time.UTC}
	if tz != "" {
		loc, err := time.LoadLocation(tz)
		if err != nil {
			return rng, apperr.Validation("Invalid timezone")
		}
		rng.Location = loc
	}
	parse := func(name, v string) (*time.Time, error) {
		if v == "" {
			return nil, nil
		}
		ts, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return nil, apperr.Validation("Invalid " + name + " date; use RFC 3339, e.g. 2026-11-21T00:00:00Z")
		}
		return &ts, nil
	}
	var err error
	if rng.From, err = parse("from", from); err != nil {
		return rng, err
	}
	if rng.To, err = parse("to", to); err != nil {
		return rng, err
	}
	if rng.From != nil && rng.To != nil && !rng.From.Before(*rng.To) {
		return rng, apperr.Validation("'from' must be before 'to'")
	}
	return rng, nil
}

// trendBucket picks a granularity that keeps the chart readable.
func trendBucket(rng domain.ReportRange, now time.Time) string {
	if rng.From == nil {
		return "month"
	}
	to := now
	if rng.To != nil {
		to = *rng.To
	}
	switch span := to.Sub(*rng.From); {
	case span <= 48*time.Hour:
		return "hour"
	case span <= 92*24*time.Hour:
		return "day"
	default:
		return "month"
	}
}

func truncate(ts time.Time, bucket string, loc *time.Location) time.Time {
	ts = ts.In(loc)
	switch bucket {
	case "hour":
		return time.Date(ts.Year(), ts.Month(), ts.Day(), ts.Hour(), 0, 0, 0, loc)
	case "month":
		return time.Date(ts.Year(), ts.Month(), 1, 0, 0, 0, 0, loc)
	default:
		return time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, loc)
	}
}

func next(ts time.Time, bucket string) time.Time {
	switch bucket {
	case "hour":
		return ts.Add(time.Hour)
	case "month":
		return ts.AddDate(0, 1, 0)
	default:
		return ts.AddDate(0, 0, 1)
	}
}

// fillTrend adds empty buckets so the chart has no gaps.
func fillTrend(points []domain.TrendPoint, start, end time.Time, bucket string) []domain.TrendPoint {
	byBucket := map[int64]domain.TrendPoint{}
	for _, p := range points {
		byBucket[p.Bucket.Unix()] = p
	}
	out := []domain.TrendPoint{}
	for b := start; !b.After(end) && len(out) < maxTrendBuckets; b = next(b, bucket) {
		if p, ok := byBucket[b.Unix()]; ok {
			out = append(out, p)
		} else {
			out = append(out, domain.TrendPoint{Bucket: b})
		}
	}
	return out
}

// Summary builds dashboard stats for tickets the actor can see within the range.
func (s *TicketService) Summary(ctx context.Context, actor domain.Actor, rng domain.ReportRange) (domain.Summary, error) {
	f := domain.TicketFilter{CreatedFrom: rng.From, CreatedTo: rng.To}
	applyScope(actor, &f)

	counts, err := s.repo.CountTickets(ctx, f)
	if err != nil {
		return domain.Summary{}, err
	}
	sum := domain.Summary{
		From: rng.From, To: rng.To,
		ByStatus:   zeroCounts(domain.Statuses),
		BySeverity: zeroCounts(domain.Severities),
		ByType:     zeroCounts(domain.Types),
	}
	for _, c := range counts {
		sum.Total += c.Count
		sum.ByStatus[c.Status] += c.Count
		sum.BySeverity[c.Severity] += c.Count
		sum.ByType[c.Type] += c.Count
	}

	if sum.ResolutionStats, err = s.repo.ResolutionStats(ctx, f); err != nil {
		return domain.Summary{}, err
	}

	now := s.now()
	sum.Bucket = trendBucket(rng, now)
	trend, err := s.repo.TicketTrend(ctx, f, sum.Bucket, rng.Location)
	if err != nil {
		return domain.Summary{}, err
	}
	switch {
	case rng.From != nil:
		end := now
		if rng.To != nil && rng.To.Before(now) {
			end = rng.To.Add(-time.Nanosecond)
		}
		sum.Trend = fillTrend(trend, truncate(*rng.From, sum.Bucket, rng.Location), truncate(end, sum.Bucket, rng.Location), sum.Bucket)
	case len(trend) > 0:
		sum.Trend = fillTrend(trend, trend[0].Bucket, trend[len(trend)-1].Bucket, sum.Bucket)
	default:
		sum.Trend = []domain.TrendPoint{}
	}

	recent := domain.TicketFilter{RequesterID: f.RequesterID, AssigneeID: f.AssigneeID, Sort: "updated", Page: 1, PageSize: recentLimit}
	sum.Recent, _, err = s.repo.ListTickets(ctx, recent)
	return sum, err
}

// ExportTickets returns tickets created in the range for CSV export (agents and admins only).
func (s *TicketService) ExportTickets(ctx context.Context, actor domain.Actor, rng domain.ReportRange) ([]domain.Ticket, error) {
	if !actor.Role.IsStaff() {
		return nil, apperr.Forbidden("Only agents and admins can export reports")
	}
	f := domain.TicketFilter{CreatedFrom: rng.From, CreatedTo: rng.To, Sort: "oldest", Page: 1, PageSize: maxExportRows}
	applyScope(actor, &f)
	tickets, _, err := s.repo.ListTickets(ctx, f)
	return tickets, err
}
