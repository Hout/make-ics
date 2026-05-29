package caldav

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	ical "github.com/emersion/go-ical"
	davlib "github.com/emersion/go-webdav/caldav"

	"github.com/jeroen/make-ics-go/pkg/pipeline"
)

type remoteEntry struct {
	href  string
	hash  string
	start time.Time
	end   time.Time
	title string
	desc  string
	year  int
}

// SyncOptions controls Sync behaviour.
type SyncOptions struct {
	// IncludePast, when true, includes events whose DtStart is before now.
	// Default (false) skips past events entirely.
	IncludePast bool
	// Start and End optionally narrow the import to a selected time period.
	// Zero values mean the corresponding bound is not applied.
	Start time.Time
	End   time.Time
	// Year scopes remote cleanup to a single import year. When zero, Sync derives
	// the year from the first active event.
	Year int
	// DryRun reports what would happen without issuing any PUT or DELETE.
	DryRun bool
	// Now overrides the "current time" reference used for past/future
	// filtering. Zero means time.Now() is used. Useful in tests.
	Now time.Time
	// Logf is an optional logger used for detailed sync traces.
	// When nil, no sync trace logs are emitted.
	Logf func(format string, a ...any)
}

// SyncResult summarises the outcome of a Sync call.
type SyncResult struct {
	Added                     int
	Updated                   int
	Unchanged                 int
	Deleted                   int
	Skipped                   int     // events skipped due to IncludePast=false
	ExcludedFromDeletionScope int     // remote managed events ignored for delete planning
	Errors                    []error // non-fatal per-event errors; sync continues on each
}

type updateEntry struct {
	event  pipeline.Event
	href   string
	before remoteEntry
}

type metadataChange struct {
	field string
	from  string
	to    string
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
				plan.toUpdate = append(plan.toUpdate, updateEntry{event: e, href: rem.href, before: rem})
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

// ListManaged queries the remote calendar inside [start, end) and returns all
// managed events, keyed by UID.
func (c *Client) ListManaged(ctx context.Context, start, end time.Time) (map[string]remoteEntry, error) {
	objects, err := c.queryManagedObjects(ctx, start, end, true)
	if err != nil {
		// Some servers are strict about property filters on custom X-props.
		// Fall back to a broader query and keep the client-side managed check.
		objects, err = c.queryManagedObjects(ctx, start, end, false)
		if err != nil {
			return nil, fmt.Errorf("caldav query: %w", err)
		}
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
		eventStart, eventEnd := extractEventWindow(obj.Data)
		eventTitle, eventDescription := extractEventText(obj.Data)
		result[uid] = remoteEntry{
			href:  obj.Path,
			hash:  objectHash(obj.Data),
			start: eventStart,
			end:   eventEnd,
			title: eventTitle,
			desc:  eventDescription,
			year:  eventStart.Year(),
		}
	}
	return result, nil
}

func (c *Client) queryManagedObjects(ctx context.Context, start, end time.Time, useManagedPropFilter bool) ([]davlib.CalendarObject, error) {
	eventFilter := davlib.CompFilter{
		Name:  ical.CompEvent,
		Start: start,
		End:   end,
	}
	if useManagedPropFilter {
		eventFilter.Props = []davlib.PropFilter{{
			Name:      managedProp,
			TextMatch: &davlib.TextMatch{Text: "1"},
		}}
	}

	query := &davlib.CalendarQuery{
		CompRequest: davlib.CalendarCompRequest{
			Name: ical.CompCalendar,
			Comps: []davlib.CalendarCompRequest{{
				Name:  ical.CompEvent,
				Props: []string{ical.PropUID, ical.PropSummary, ical.PropDescription, ical.PropDateTimeStart, ical.PropDateTimeEnd, managedProp},
			}},
		},
		CompFilter: davlib.CompFilter{
			Name:  ical.CompCalendar,
			Comps: []davlib.CompFilter{eventFilter},
		},
	}

	return c.dav.QueryCalendar(ctx, c.calendarURL, query)
}

// Sync synchronises events to the remote CalDAV calendar. It creates missing
// events, updates changed ones, and deletes managed events that are no longer
// in the input. User-created events (without X-MAKE-ICS-MANAGED:1) are never
// touched.
//
// By default (SyncOptions.IncludePast=false) events whose DtStart is before
// time.Now() are skipped and counted in SyncResult.Skipped. Remote cleanup is
// scoped to the selected year and current sync window. This avoids deleting
// managed historical events that were intentionally skipped from the desired
// set.
func (c *Client) Sync(ctx context.Context, events []pipeline.Event, opts SyncOptions) (SyncResult, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	var active []pipeline.Event
	var skipped int
	for _, e := range events {
		if !opts.IncludePast && e.DtStart.Before(now) {
			skipped++
			continue
		}
		if !eventInRange(e, opts.Start, opts.End) {
			continue
		}
		active = append(active, e)
	}

	var result SyncResult
	result.Skipped = skipped

	year := opts.Year
	if year == 0 && len(active) > 0 {
		year = active[0].DtStart.Year()
	}
	if year == 0 {
		return result, nil
	}

	queryStart := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
	queryEnd := queryStart.AddDate(1, 0, 0)

	fetchStarted := time.Now()
	if opts.Logf != nil {
		opts.Logf("remote scan: querying managed events in year=%d window [%s, %s)",
			year,
			queryStart.Format(time.RFC3339),
			queryEnd.Format(time.RFC3339),
		)
	}
	remote, err := c.ListManaged(ctx, queryStart, queryEnd)
	if err != nil {
		return result, err
	}
	if opts.Logf != nil {
		opts.Logf("remote scan: fetched %d managed events in %s", len(remote), time.Since(fetchStarted))
	}

	remote = filterRemoteByYear(remote, year)
	if opts.Logf != nil {
		opts.Logf("remote scan: %d managed events remain after year=%d filter", len(remote), year)
	}

	remoteBeforeDeletionScope := len(remote)
	remote = filterRemoteForDeletionScope(remote, now, opts)
	result.ExcludedFromDeletionScope = remoteBeforeDeletionScope - len(remote)
	if opts.Logf != nil {
		opts.Logf("remote scan: %d managed events excluded from deletion scope", result.ExcludedFromDeletionScope)
		opts.Logf("remote scan: %d managed events remain after deletion-scope filter", len(remote))
	}

	plan := makePlan(remote, active, now, SyncOptions{IncludePast: true})
	sortSyncPlanChronological(&plan)
	result.Unchanged = plan.unchanged
	if opts.Logf != nil {
		opts.Logf("plan: add=%d update=%d delete=%d unchanged=%d skipped=%d",
			len(plan.toAdd), len(plan.toUpdate), len(plan.toDelete), plan.unchanged, result.Skipped)
	}

	for _, e := range plan.toAdd {
		if opts.DryRun {
			if opts.Logf != nil {
				opts.Logf("would write appointment summary=%q start=%s end=%s",
					e.Summary,
					e.DtStart.Format(time.RFC3339),
					e.DtEnd.Format(time.RFC3339),
				)
			}
		}
		if !opts.DryRun {
			href := c.calendarURL + e.UID + ".ics"
			if _, err := c.dav.PutCalendarObject(ctx, href, buildCalObject(e)); err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("add %s: %w", e.UID, err))
				continue
			}
			if opts.Logf != nil {
				opts.Logf("wrote appointment summary=%q start=%s end=%s",
					e.Summary,
					e.DtStart.Format(time.RFC3339),
					e.DtEnd.Format(time.RFC3339),
				)
			}
		}
		result.Added++
	}

	for _, u := range plan.toUpdate {
		changes := diffEventMetadata(u.before, u.event)
		changeSummary := formatMetadataChanges(changes)
		changeJSON := formatMetadataChangesJSON(changes)
		if opts.DryRun {
			if opts.Logf != nil {
				opts.Logf("would change appointment changes=%s changes_json=%s summary=%q start=%s end=%s",
					changeSummary,
					changeJSON,
					u.event.Summary,
					u.event.DtStart.Format(time.RFC3339),
					u.event.DtEnd.Format(time.RFC3339),
				)
			}
		}
		if !opts.DryRun {
			if _, err := c.dav.PutCalendarObject(ctx, u.href, buildCalObject(u.event)); err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("update %s: %w", u.event.UID, err))
				continue
			}
			if opts.Logf != nil {
				opts.Logf("changed appointment changes=%s changes_json=%s summary=%q start=%s end=%s",
					changeSummary,
					changeJSON,
					u.event.Summary,
					u.event.DtStart.Format(time.RFC3339),
					u.event.DtEnd.Format(time.RFC3339),
				)
			}
		}
		result.Updated++
	}

	for _, rem := range plan.toDelete {
		if opts.DryRun {
			if opts.Logf != nil {
				opts.Logf("would delete appointment start=%s end=%s",
					formatOptionalTime(rem.start),
					formatOptionalTime(rem.end),
				)
			}
		}
		if !opts.DryRun {
			if err := c.dav.RemoveAll(ctx, rem.href); err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("delete %s: %w", rem.href, err))
				continue
			}
			if opts.Logf != nil {
				opts.Logf("deleted appointment start=%s end=%s",
					formatOptionalTime(rem.start),
					formatOptionalTime(rem.end),
				)
			}
		}
		result.Deleted++
	}

	return result, nil
}

func sortSyncPlanChronological(plan *syncPlan) {
	sort.Slice(plan.toAdd, func(i, j int) bool {
		return eventLess(plan.toAdd[i], plan.toAdd[j])
	})

	sort.Slice(plan.toUpdate, func(i, j int) bool {
		return eventLess(plan.toUpdate[i].event, plan.toUpdate[j].event)
	})

	sort.Slice(plan.toDelete, func(i, j int) bool {
		left := plan.toDelete[i]
		right := plan.toDelete[j]

		if !left.start.Equal(right.start) {
			if left.start.IsZero() {
				return false
			}
			if right.start.IsZero() {
				return true
			}
			return left.start.Before(right.start)
		}
		if !left.end.Equal(right.end) {
			if left.end.IsZero() {
				return false
			}
			if right.end.IsZero() {
				return true
			}
			return left.end.Before(right.end)
		}
		return left.href < right.href
	})
}

func eventLess(left, right pipeline.Event) bool {
	if !left.DtStart.Equal(right.DtStart) {
		return left.DtStart.Before(right.DtStart)
	}
	if !left.DtEnd.Equal(right.DtEnd) {
		return left.DtEnd.Before(right.DtEnd)
	}
	if left.Summary != right.Summary {
		return left.Summary < right.Summary
	}
	return left.UID < right.UID
}

func eventInRange(e pipeline.Event, start, end time.Time) bool {
	if !start.IsZero() && e.DtEnd.Before(start) {
		return false
	}
	if !end.IsZero() && e.DtStart.After(end) {
		return false
	}
	return true
}

func filterRemoteByYear(remote map[string]remoteEntry, year int) map[string]remoteEntry {
	if year == 0 {
		return remote
	}
	filtered := make(map[string]remoteEntry, len(remote))
	for uid, rem := range remote {
		if rem.year == year {
			filtered[uid] = rem
		}
	}
	return filtered
}

func filterRemoteForDeletionScope(remote map[string]remoteEntry, now time.Time, opts SyncOptions) map[string]remoteEntry {
	filtered := make(map[string]remoteEntry, len(remote))
	for uid, rem := range remote {
		if !remoteEntryInDeletionScope(rem, now, opts) {
			continue
		}
		filtered[uid] = rem
	}
	return filtered
}

func remoteEntryInDeletionScope(rem remoteEntry, now time.Time, opts SyncOptions) bool {
	if !opts.IncludePast {
		if eventTime := effectiveRemoteEventTime(rem); !eventTime.IsZero() && eventTime.Before(now) {
			return false
		}
	}

	if !opts.Start.IsZero() {
		if rem.end.IsZero() || rem.end.Before(opts.Start) {
			return false
		}
	}

	if !opts.End.IsZero() {
		if rem.start.IsZero() || rem.start.After(opts.End) {
			return false
		}
	}

	return true
}

func effectiveRemoteEventTime(rem remoteEntry) time.Time {
	if !rem.end.IsZero() {
		return rem.end
	}
	return rem.start
}

func extractEventWindow(cal *ical.Calendar) (time.Time, time.Time) {
	events := cal.Events()
	if len(events) == 0 {
		return time.Time{}, time.Time{}
	}
	start, _ := events[0].Props.DateTime(ical.PropDateTimeStart, time.UTC)
	end, _ := events[0].Props.DateTime(ical.PropDateTimeEnd, time.UTC)
	return start, end
}

func extractEventText(cal *ical.Calendar) (string, string) {
	events := cal.Events()
	if len(events) == 0 {
		return "", ""
	}
	title, _ := events[0].Props.Text(ical.PropSummary)
	description, _ := events[0].Props.Text(ical.PropDescription)
	return title, description
}

func diffEventMetadata(before remoteEntry, after pipeline.Event) []metadataChange {
	changes := make([]metadataChange, 0, 4)

	if before.title != after.Summary {
		changes = append(changes, metadataChange{field: "title", from: before.title, to: after.Summary})
	}

	if before.desc != after.Description {
		changes = append(changes, metadataChange{field: "description", from: before.desc, to: after.Description})
	}

	if dateChanged(before.start, after.DtStart) || dateChanged(before.end, after.DtEnd) {
		changes = append(changes, metadataChange{
			field: "date",
			from:  formatDateRange(before.start, before.end),
			to:    formatDateRange(after.DtStart, after.DtEnd),
		})
	}

	if timeChanged(before.start, after.DtStart) || timeChanged(before.end, after.DtEnd) {
		changes = append(changes, metadataChange{
			field: "time",
			from:  formatClockRange(before.start, before.end),
			to:    formatClockRange(after.DtStart, after.DtEnd),
		})
	}

	return changes
}

func formatMetadataChanges(changes []metadataChange) string {
	if len(changes) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(changes))
	for _, change := range changes {
		parts = append(parts, fmt.Sprintf("%s:%q->%q", change.field, change.from, change.to))
	}
	return strings.Join(parts, "; ")
}

func formatMetadataChangesJSON(changes []metadataChange) string {
	if len(changes) == 0 {
		return "[]"
	}

	type metadataChangePayload struct {
		Field string `json:"field"`
		From  string `json:"from"`
		To    string `json:"to"`
	}

	payload := make([]metadataChangePayload, 0, len(changes))
	for _, change := range changes {
		payload = append(payload, metadataChangePayload{
			Field: change.field,
			From:  change.from,
			To:    change.to,
		})
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

func dateChanged(left, right time.Time) bool {
	if left.IsZero() || right.IsZero() {
		return !left.Equal(right)
	}
	return left.Year() != right.Year() || left.Month() != right.Month() || left.Day() != right.Day()
}

func timeChanged(left, right time.Time) bool {
	if left.IsZero() || right.IsZero() {
		return !left.Equal(right)
	}
	return left.Hour() != right.Hour() || left.Minute() != right.Minute() || left.Second() != right.Second()
}

func formatDateRange(start, end time.Time) string {
	if start.IsZero() || end.IsZero() {
		return "unknown"
	}
	return fmt.Sprintf("%s..%s", start.Format("2006-01-02"), end.Format("2006-01-02"))
}

func formatClockRange(start, end time.Time) string {
	if start.IsZero() || end.IsZero() {
		return "unknown"
	}
	return fmt.Sprintf("%s..%s", start.Format("15:04:05"), end.Format("15:04:05"))
}

func formatOptionalTime(v time.Time) string {
	if v.IsZero() {
		return "unknown"
	}
	return v.Format(time.RFC3339)
}
