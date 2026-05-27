package caldav

import (
	"context"
	"fmt"
	"time"

	ical "github.com/emersion/go-ical"
	davlib "github.com/emersion/go-webdav/caldav"

	"github.com/jeroen/make-ics-go/pkg/pipeline"
)

type remoteEntry struct {
	href string
	hash string
}

// SyncOptions controls Sync behaviour.
type SyncOptions struct {
	// IncludePast, when true, includes events whose DtStart is before now.
	// Default (false) skips past events entirely.
	IncludePast bool
	// DryRun reports what would happen without issuing any PUT or DELETE.
	DryRun bool
	// Now overrides the "current time" reference used for past/future
	// filtering. Zero means time.Now() is used. Useful in tests.
	Now time.Time
}

// SyncResult summarises the outcome of a Sync call.
type SyncResult struct {
	Added     int
	Updated   int
	Unchanged int
	Deleted   int
	Skipped   int     // events skipped due to IncludePast=false
	Errors    []error // non-fatal per-event errors; sync continues on each
}

type updateEntry struct {
	event pipeline.Event
	href  string
}

type syncPlan struct {
	toAdd     []pipeline.Event
	toUpdate  []updateEntry
	toDelete  []remoteEntry
	unchanged int
	skipped   int
}

// makePlan computes the diff between remote and desired events without any IO.
// It is a pure function and is the primary unit under test.
func makePlan(remote map[string]remoteEntry, events []pipeline.Event, now time.Time, opts SyncOptions) syncPlan {
	var plan syncPlan
	desired := make(map[string]pipeline.Event, len(events))

	for _, e := range events {
		if !opts.IncludePast && e.DtStart.Before(now) {
			plan.skipped++
			continue
		}
		desired[e.UID] = e
	}

	for uid, e := range desired {
		wantHash := eventHash(e)
		if rem, ok := remote[uid]; ok {
			if rem.hash == wantHash {
				plan.unchanged++
			} else {
				plan.toUpdate = append(plan.toUpdate, updateEntry{e, rem.href})
			}
		} else {
			plan.toAdd = append(plan.toAdd, e)
		}
	}

	for uid, rem := range remote {
		if _, ok := desired[uid]; !ok {
			plan.toDelete = append(plan.toDelete, rem)
		}
	}

	return plan
}

// ListManaged queries the remote calendar for events in [start, end) and
// returns those carrying X-MAKE-ICS-MANAGED:1, keyed by UID.
func (c *Client) ListManaged(ctx context.Context, start, end time.Time) (map[string]remoteEntry, error) {
	query := &davlib.CalendarQuery{
		CompRequest: davlib.CalendarCompRequest{
			Name:     ical.CompCalendar,
			AllProps: true,
			AllComps: true,
		},
		CompFilter: davlib.CompFilter{
			Name: ical.CompCalendar,
			Comps: []davlib.CompFilter{{
				Name:  ical.CompEvent,
				Start: start,
				End:   end,
			}},
		},
	}

	objects, err := c.dav.QueryCalendar(ctx, c.calendarURL, query)
	if err != nil {
		return nil, fmt.Errorf("caldav query: %w", err)
	}

	result := make(map[string]remoteEntry, len(objects))
	for _, obj := range objects {
		if obj.Data == nil || !isManaged(obj.Data) {
			continue
		}
		uid := extractUID(obj.Data)
		if uid == "" {
			continue
		}
		result[uid] = remoteEntry{
			href: obj.Path,
			hash: objectHash(obj.Data),
		}
	}
	return result, nil
}

// Sync synchronises events to the remote CalDAV calendar. It creates missing
// events, updates changed ones, and deletes managed events that are no longer
// in the input. User-created events (without X-MAKE-ICS-MANAGED:1) are never
// touched.
//
// By default (SyncOptions.IncludePast=false) events whose DtStart is before
// time.Now() are skipped and counted in SyncResult.Skipped. Past events
// already on the server are also left untouched: only events in the computed
// future window are queried and reconciled.
func (c *Client) Sync(ctx context.Context, events []pipeline.Event, opts SyncOptions) (SyncResult, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	plan := makePlan(nil, events, now, opts) // first pass: just count skipped
	var result SyncResult
	result.Skipped = plan.skipped

	// Build the list of events that will actually be synced.
	var active []pipeline.Event
	for _, e := range events {
		if !opts.IncludePast && e.DtStart.Before(now) {
			continue
		}
		active = append(active, e)
	}

	if len(active) == 0 {
		return result, nil
	}

	start, end := windowBounds(active)
	remote, err := c.ListManaged(ctx, start, end)
	if err != nil {
		return result, err
	}

	// Diff against remote state. Pass IncludePast=true because active is
	// already filtered.
	plan = makePlan(remote, active, now, SyncOptions{IncludePast: true})
	result.Unchanged = plan.unchanged

	for _, e := range plan.toAdd {
		if !opts.DryRun {
			href := c.calendarURL + e.UID + ".ics"
			if _, err := c.dav.PutCalendarObject(ctx, href, buildCalObject(e)); err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("add %s: %w", e.UID, err))
				continue
			}
		}
		result.Added++
	}

	for _, u := range plan.toUpdate {
		if !opts.DryRun {
			if _, err := c.dav.PutCalendarObject(ctx, u.href, buildCalObject(u.event)); err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("update %s: %w", u.event.UID, err))
				continue
			}
		}
		result.Updated++
	}

	for _, rem := range plan.toDelete {
		if !opts.DryRun {
			if err := c.dav.RemoveAll(ctx, rem.href); err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("delete %s: %w", rem.href, err))
				continue
			}
		}
		result.Deleted++
	}

	return result, nil
}

func windowBounds(events []pipeline.Event) (start, end time.Time) {
	start = events[0].DtStart
	end = events[0].DtEnd
	for _, e := range events[1:] {
		if e.DtStart.Before(start) {
			start = e.DtStart
		}
		if e.DtEnd.After(end) {
			end = e.DtEnd
		}
	}
	return
}
