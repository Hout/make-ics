package caldav

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	ical "github.com/emersion/go-ical"
	davlib "github.com/emersion/go-webdav/caldav"

	"github.com/jeroen/make-ics-go/pkg/model"
	"github.com/jeroen/make-ics-go/pkg/pipeline"
)

// ---- pure unit tests for makePlan ----

var anchor = time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)

func futureEvent(uid string) pipeline.Event {
	return pipeline.Event{
		UID:         uid,
		Summary:     "S",
		Description: "d",
		DtStart:     anchor.Add(24 * time.Hour),
		DtEnd:       anchor.Add(26 * time.Hour),
	}
}

func pastEvent(uid string) pipeline.Event {
	e := futureEvent(uid)
	e.DtStart = anchor.Add(-48 * time.Hour)
	e.DtEnd = anchor.Add(-46 * time.Hour)
	return e
}

func TestMakePlan_AllNew(t *testing.T) {
	events := []pipeline.Event{futureEvent("a"), futureEvent("b")}
	plan := makePlan(nil, events, anchor, SyncOptions{IncludePast: true})
	if len(plan.toAdd) != 2 {
		t.Errorf("toAdd=%d want 2", len(plan.toAdd))
	}
	if len(plan.toUpdate) != 0 || len(plan.toDelete) != 0 {
		t.Errorf("unexpected update/delete in all-new plan")
	}
}

func TestMakePlan_Unchanged(t *testing.T) {
	e := futureEvent("a")
	remote := map[string]remoteEntry{"a": {href: "/cal/a.ics", hash: eventHash(e)}}
	plan := makePlan(remote, []pipeline.Event{e}, anchor, SyncOptions{IncludePast: true})
	if plan.unchanged != 1 || len(plan.toAdd) != 0 || len(plan.toUpdate) != 0 {
		t.Errorf("expected unchanged=1, got add=%d update=%d unchanged=%d",
			len(plan.toAdd), len(plan.toUpdate), plan.unchanged)
	}
}

func TestMakePlan_Update(t *testing.T) {
	e := futureEvent("a")
	modified := e
	modified.Summary = "Changed"
	remote := map[string]remoteEntry{"a": {href: "/cal/a.ics", hash: eventHash(e)}}
	plan := makePlan(remote, []pipeline.Event{modified}, anchor, SyncOptions{IncludePast: true})
	if len(plan.toUpdate) != 1 {
		t.Errorf("toUpdate=%d want 1", len(plan.toUpdate))
	}
}

func TestMakePlan_Delete(t *testing.T) {
	remote := map[string]remoteEntry{"old": {href: "/cal/old.ics", hash: "h"}}
	plan := makePlan(remote, nil, anchor, SyncOptions{IncludePast: true})
	if len(plan.toDelete) != 1 {
		t.Errorf("toDelete=%d want 1", len(plan.toDelete))
	}
}

func TestMakePlan_SkipPast(t *testing.T) {
	events := []pipeline.Event{futureEvent("f"), pastEvent("p")}
	plan := makePlan(nil, events, anchor, SyncOptions{IncludePast: false})
	if plan.skipped != 1 {
		t.Errorf("skipped=%d want 1", plan.skipped)
	}
	if len(plan.toAdd) != 1 || plan.toAdd[0].UID != "f" {
		t.Errorf("expected only future event in toAdd")
	}
}

func TestMakePlan_IncludePast(t *testing.T) {
	events := []pipeline.Event{futureEvent("f"), pastEvent("p")}
	plan := makePlan(nil, events, anchor, SyncOptions{IncludePast: true})
	if plan.skipped != 0 || len(plan.toAdd) != 2 {
		t.Errorf("expected all events added when IncludePast=true, got skipped=%d add=%d",
			plan.skipped, len(plan.toAdd))
	}
}

func TestMakePlan_PastEventsSkipped(t *testing.T) {
	events := []pipeline.Event{futureEvent("f"), pastEvent("p")}
	plan := makePlan(nil, events, anchor, SyncOptions{IncludePast: false})
	if plan.skipped != 1 {
		t.Errorf("skipped=%d want 1", plan.skipped)
	}
	if len(plan.toAdd) != 1 || plan.toAdd[0].UID != "f" {
		t.Errorf("expected only future event in toAdd")
	}
}

func TestSortSyncPlanChronological(t *testing.T) {
	late := futureEvent("late")
	late.DtStart = anchor.Add(4 * time.Hour)
	late.DtEnd = anchor.Add(5 * time.Hour)

	early := futureEvent("early")
	early.DtStart = anchor.Add(1 * time.Hour)
	early.DtEnd = anchor.Add(2 * time.Hour)

	mid := futureEvent("mid")
	mid.DtStart = anchor.Add(2 * time.Hour)
	mid.DtEnd = anchor.Add(3 * time.Hour)

	plan := syncPlan{
		toAdd: []pipeline.Event{late, early, mid},
		toUpdate: []updateEntry{
			{event: late, href: "/cal/late.ics"},
			{event: early, href: "/cal/early.ics"},
			{event: mid, href: "/cal/mid.ics"},
		},
		toDelete: []remoteEntry{
			{href: "/cal/late.ics", start: late.DtStart, end: late.DtEnd},
			{href: "/cal/early.ics", start: early.DtStart, end: early.DtEnd},
			{href: "/cal/mid.ics", start: mid.DtStart, end: mid.DtEnd},
		},
	}

	sortSyncPlanChronological(&plan)

	if got := plan.toAdd[0].UID; got != "early" {
		t.Fatalf("toAdd[0] = %q, want %q", got, "early")
	}
	if got := plan.toAdd[1].UID; got != "mid" {
		t.Fatalf("toAdd[1] = %q, want %q", got, "mid")
	}
	if got := plan.toAdd[2].UID; got != "late" {
		t.Fatalf("toAdd[2] = %q, want %q", got, "late")
	}

	if got := plan.toUpdate[0].event.UID; got != "early" {
		t.Fatalf("toUpdate[0] = %q, want %q", got, "early")
	}
	if got := plan.toUpdate[1].event.UID; got != "mid" {
		t.Fatalf("toUpdate[1] = %q, want %q", got, "mid")
	}
	if got := plan.toUpdate[2].event.UID; got != "late" {
		t.Fatalf("toUpdate[2] = %q, want %q", got, "late")
	}

	if got := plan.toDelete[0].href; got != "/cal/early.ics" {
		t.Fatalf("toDelete[0] = %q, want %q", got, "/cal/early.ics")
	}
	if got := plan.toDelete[1].href; got != "/cal/mid.ics" {
		t.Fatalf("toDelete[1] = %q, want %q", got, "/cal/mid.ics")
	}
	if got := plan.toDelete[2].href; got != "/cal/late.ics" {
		t.Fatalf("toDelete[2] = %q, want %q", got, "/cal/late.ics")
	}
}

// ---- integration test: Sync against a fake CalDAV server ----

const (
	fakePrincipal = "/user/"
	fakeHomeSet   = "/user/calendars/"
	fakeCalPath   = "/user/calendars/test/"
	fakeCalName   = "Test Calendar"
)

type fakeBackend struct {
	mu      sync.Mutex
	objects map[string]*ical.Calendar
	puts    []string
	deletes []string
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{objects: make(map[string]*ical.Calendar)}
}

func (b *fakeBackend) CurrentUserPrincipal(_ context.Context) (string, error) {
	return fakePrincipal, nil
}
func (b *fakeBackend) CalendarHomeSetPath(_ context.Context) (string, error) {
	return fakeHomeSet, nil
}
func (b *fakeBackend) CreateCalendar(_ context.Context, _ *davlib.Calendar) error { return nil }
func (b *fakeBackend) ListCalendars(_ context.Context) ([]davlib.Calendar, error) {
	return []davlib.Calendar{{Path: fakeCalPath, Name: fakeCalName}}, nil
}
func (b *fakeBackend) GetCalendar(_ context.Context, path string) (*davlib.Calendar, error) {
	if path == fakeCalPath {
		return &davlib.Calendar{Path: fakeCalPath, Name: fakeCalName}, nil
	}
	return nil, fmt.Errorf("not found: %s", path)
}
func (b *fakeBackend) GetCalendarObject(_ context.Context, path string, _ *davlib.CalendarCompRequest) (*davlib.CalendarObject, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	cal, ok := b.objects[path]
	if !ok {
		return nil, fmt.Errorf("not found: %s", path)
	}
	return &davlib.CalendarObject{Path: path, Data: cal}, nil
}
func (b *fakeBackend) ListCalendarObjects(_ context.Context, path string, _ *davlib.CalendarCompRequest) ([]davlib.CalendarObject, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []davlib.CalendarObject
	for p, cal := range b.objects {
		if strings.HasPrefix(p, path) {
			out = append(out, davlib.CalendarObject{Path: p, Data: cal})
		}
	}
	return out, nil
}
func (b *fakeBackend) QueryCalendarObjects(_ context.Context, path string, _ *davlib.CalendarQuery) ([]davlib.CalendarObject, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []davlib.CalendarObject
	for p, cal := range b.objects {
		if strings.HasPrefix(p, path) {
			out = append(out, davlib.CalendarObject{Path: p, Data: cal})
		}
	}
	return out, nil
}
func (b *fakeBackend) PutCalendarObject(_ context.Context, path string, cal *ical.Calendar, _ *davlib.PutCalendarObjectOptions) (*davlib.CalendarObject, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.objects[path] = cal
	b.puts = append(b.puts, path)
	return &davlib.CalendarObject{Path: path, ETag: fmt.Sprintf(`"etag-%d"`, len(b.puts))}, nil
}
func (b *fakeBackend) DeleteCalendarObject(_ context.Context, path string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.objects, path)
	b.deletes = append(b.deletes, path)
	return nil
}

func newTestClient(t *testing.T, backend *fakeBackend) (*Client, *httptest.Server) {
	t.Helper()
	handler := &davlib.Handler{Backend: backend}
	ts := httptest.NewServer(handler)
	cfg := model.CalDAVConfig{
		URL:                 ts.URL,
		CalendarDisplayName: fakeCalName,
	}
	client, err := New(context.Background(), cfg, "", "")
	if err != nil {
		ts.Close()
		t.Fatalf("New: %v", err)
	}
	return client, ts
}

func TestSync_FirstRun_CreatesEvents(t *testing.T) {
	backend := newFakeBackend()
	client, ts := newTestClient(t, backend)
	defer ts.Close()

	events := []pipeline.Event{futureEvent("ev1"), futureEvent("ev2")}
	res, err := client.Sync(context.Background(), events, SyncOptions{IncludePast: true})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Added != 2 || res.Updated != 0 || res.Deleted != 0 {
		t.Errorf("want add=2 update=0 del=0, got %+v", res)
	}
	if len(backend.puts) != 2 {
		t.Errorf("want 2 PUTs, got %d", len(backend.puts))
	}
}

func TestSync_Idempotent_NoOp(t *testing.T) {
	backend := newFakeBackend()
	client, ts := newTestClient(t, backend)
	defer ts.Close()

	events := []pipeline.Event{futureEvent("ev1")}
	opts := SyncOptions{IncludePast: true}

	if _, err := client.Sync(context.Background(), events, opts); err != nil {
		t.Fatalf("first Sync: %v", err)
	}
	backend.puts = nil // reset call log

	res, err := client.Sync(context.Background(), events, opts)
	if err != nil {
		t.Fatalf("second Sync: %v", err)
	}
	if res.Added != 0 || res.Updated != 0 || res.Unchanged != 1 {
		t.Errorf("second run not idempotent: %+v", res)
	}
	if len(backend.puts) != 0 {
		t.Errorf("second run issued %d PUTs, want 0", len(backend.puts))
	}
}

func TestSync_Update(t *testing.T) {
	backend := newFakeBackend()
	client, ts := newTestClient(t, backend)
	defer ts.Close()

	e := futureEvent("ev1")
	opts := SyncOptions{IncludePast: true}
	if _, err := client.Sync(context.Background(), []pipeline.Event{e}, opts); err != nil {
		t.Fatalf("first Sync: %v", err)
	}
	backend.puts = nil

	e.Summary = "Updated Summary"
	res, err := client.Sync(context.Background(), []pipeline.Event{e}, opts)
	if err != nil {
		t.Fatalf("second Sync: %v", err)
	}
	if res.Updated != 1 || res.Added != 0 {
		t.Errorf("want update=1 add=0, got %+v", res)
	}
}

func TestSync_Delete(t *testing.T) {
	backend := newFakeBackend()
	client, ts := newTestClient(t, backend)
	defer ts.Close()

	e := futureEvent("ev1")
	opts := SyncOptions{IncludePast: true}
	if _, err := client.Sync(context.Background(), []pipeline.Event{e}, opts); err != nil {
		t.Fatalf("first Sync: %v", err)
	}
	backend.deletes = nil

	// Second sync with no events → ev1 should be deleted.
	e2 := futureEvent("ev2") // keep the window alive with a different event
	if _, err := client.Sync(context.Background(), []pipeline.Event{e2}, opts); err != nil {
		t.Fatalf("second Sync: %v", err)
	}
	if len(backend.deletes) != 1 {
		t.Errorf("want 1 DELETE, got %d", len(backend.deletes))
	}
}

func TestSync_SkipPastByDefault(t *testing.T) {
	backend := newFakeBackend()
	client, ts := newTestClient(t, backend)
	defer ts.Close()

	events := []pipeline.Event{pastEvent("past"), futureEvent("future")}
	res, err := client.Sync(context.Background(), events, SyncOptions{Now: anchor})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Skipped != 1 {
		t.Errorf("skipped=%d want 1", res.Skipped)
	}
	if res.Added != 1 {
		t.Errorf("added=%d want 1 (only future event)", res.Added)
	}
}

func TestSync_IncludePast(t *testing.T) {
	backend := newFakeBackend()
	client, ts := newTestClient(t, backend)
	defer ts.Close()

	events := []pipeline.Event{pastEvent("past"), futureEvent("future")}
	res, err := client.Sync(context.Background(), events, SyncOptions{IncludePast: true, Now: anchor})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Skipped != 0 || res.Added != 2 {
		t.Errorf("IncludePast: skipped=%d added=%d, want 0 and 2", res.Skipped, res.Added)
	}
}

func TestSync_PeriodFilter(t *testing.T) {
	backend := newFakeBackend()
	client, ts := newTestClient(t, backend)
	defer ts.Close()

	early := futureEvent("early")
	early.DtStart = time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	early.DtEnd = time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)

	late := futureEvent("late")
	late.DtStart = time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC)
	late.DtEnd = time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)

	opts := SyncOptions{
		IncludePast: true,
		Start:       time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		End:         time.Date(2026, 6, 30, 23, 59, 59, 0, time.UTC),
		Year:        2026,
	}

	res, err := client.Sync(context.Background(), []pipeline.Event{early, late}, opts)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Added != 1 || res.Skipped != 0 {
		t.Errorf("period filter: %+v", res)
	}
	if len(backend.puts) != 1 {
		t.Fatalf("want 1 PUT, got %d", len(backend.puts))
	}
	if got := backend.puts[0]; !strings.Contains(got, "early") {
		t.Fatalf("unexpected PUT path %q, want early event", got)
	}
}

func TestSync_DoesNotDeleteStaleMovedEventOutsideWindow(t *testing.T) {
	backend := newFakeBackend()
	client, ts := newTestClient(t, backend)
	defer ts.Close()

	oldEvent := futureEvent("ev1")
	oldEvent.DtStart = time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	oldEvent.DtEnd = time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	if _, err := client.Sync(context.Background(), []pipeline.Event{oldEvent}, SyncOptions{IncludePast: true, Year: 2026}); err != nil {
		t.Fatalf("first Sync: %v", err)
	}
	backend.deletes = nil

	newEvent := oldEvent
	newEvent.UID = "ev2"
	newEvent.DtStart = time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	newEvent.DtEnd = time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)

	res, err := client.Sync(context.Background(), []pipeline.Event{newEvent}, SyncOptions{
		IncludePast: true,
		Start:       time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		End:         time.Date(2026, 8, 31, 23, 59, 59, 0, time.UTC),
		Year:        2026,
	})
	if err != nil {
		t.Fatalf("second Sync: %v", err)
	}
	if res.Added != 1 || res.Deleted != 0 {
		t.Fatalf("want add=1 delete=0, got %+v", res)
	}
	if len(backend.deletes) != 0 {
		t.Fatalf("want 0 DELETE, got %d", len(backend.deletes))
	}
}

func TestSync_SkippingPastDoesNotDeletePastManagedEvents(t *testing.T) {
	backend := newFakeBackend()
	client, ts := newTestClient(t, backend)
	defer ts.Close()

	past := futureEvent("past")
	past.DtStart = anchor.Add(-72 * time.Hour)
	past.DtEnd = anchor.Add(-70 * time.Hour)

	if _, err := client.Sync(context.Background(), []pipeline.Event{past}, SyncOptions{IncludePast: true, Year: past.DtStart.Year(), Now: anchor}); err != nil {
		t.Fatalf("seed Sync: %v", err)
	}

	backend.deletes = nil

	future := futureEvent("future")
	res, err := client.Sync(context.Background(), []pipeline.Event{future}, SyncOptions{IncludePast: false, Year: future.DtStart.Year(), Now: anchor})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if res.Deleted != 0 {
		t.Fatalf("want delete=0, got %+v", res)
	}
	if res.ExcludedFromDeletionScope != 1 {
		t.Fatalf("want excluded_from_deletion_scope=1, got %+v", res)
	}
	if len(backend.deletes) != 0 {
		t.Fatalf("want 0 DELETE, got %d", len(backend.deletes))
	}
	if _, ok := backend.objects[fakeCalPath+"past.ics"]; !ok {
		t.Fatalf("past managed event should remain on server")
	}
}

func TestSync_DeletesStaleFutureManagedEventsWhenSkippingPast(t *testing.T) {
	backend := newFakeBackend()
	client, ts := newTestClient(t, backend)
	defer ts.Close()

	oldFuture := futureEvent("future-old")
	oldFuture.DtStart = anchor.Add(24 * time.Hour)
	oldFuture.DtEnd = anchor.Add(26 * time.Hour)

	if _, err := client.Sync(context.Background(), []pipeline.Event{oldFuture}, SyncOptions{IncludePast: true, Year: oldFuture.DtStart.Year(), Now: anchor}); err != nil {
		t.Fatalf("seed Sync: %v", err)
	}

	backend.deletes = nil

	newFuture := futureEvent("future-new")
	newFuture.DtStart = anchor.Add(48 * time.Hour)
	newFuture.DtEnd = anchor.Add(50 * time.Hour)

	res, err := client.Sync(context.Background(), []pipeline.Event{newFuture}, SyncOptions{IncludePast: false, Year: newFuture.DtStart.Year(), Now: anchor})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if res.Deleted != 1 {
		t.Fatalf("want delete=1, got %+v", res)
	}
	if res.ExcludedFromDeletionScope != 0 {
		t.Fatalf("want excluded_from_deletion_scope=0, got %+v", res)
	}
	if len(backend.deletes) != 1 {
		t.Fatalf("want 1 DELETE, got %d", len(backend.deletes))
	}
}

func TestSync_UserCreatedEventUntouched(t *testing.T) {
	backend := newFakeBackend()

	// Pre-populate the calendar with a user-created event (no X-MAKE-ICS-MANAGED).
	userCal := ical.NewCalendar()
	userCal.Props.SetText(ical.PropProductID, "-//user//client//EN")
	userCal.Props.SetText(ical.PropVersion, "2.0")
	userEv := ical.NewEvent()
	userEv.Props.SetText(ical.PropUID, "user-uid")
	userEv.Props.SetText(ical.PropSummary, "My own event")
	userEv.Props.SetDateTime(ical.PropDateTimeStart, anchor.Add(25*time.Hour))
	userEv.Props.SetDateTime(ical.PropDateTimeEnd, anchor.Add(26*time.Hour))
	userEv.Props.SetDateTime(ical.PropDateTimeStamp, time.Now().UTC())
	userCal.Children = append(userCal.Children, userEv.Component)
	backend.objects[fakeCalPath+"user-uid.ics"] = userCal
	backend.deletes = nil

	client, ts := newTestClient(t, backend)
	defer ts.Close()

	// Sync with a different managed event in the same time window.
	managed := futureEvent("managed-uid")
	managed.DtStart = anchor.Add(25 * time.Hour)
	managed.DtEnd = anchor.Add(26 * time.Hour)

	_, err := client.Sync(context.Background(), []pipeline.Event{managed}, SyncOptions{IncludePast: true})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(backend.deletes) != 0 {
		t.Errorf("user-created event was deleted: %v", backend.deletes)
	}
	if _, ok := backend.objects[fakeCalPath+"user-uid.ics"]; !ok {
		t.Error("user-created event was removed from backend")
	}
}

func TestSync_DryRun_NoPuts(t *testing.T) {
	backend := newFakeBackend()
	client, ts := newTestClient(t, backend)
	defer ts.Close()

	events := []pipeline.Event{futureEvent("ev1"), futureEvent("ev2")}
	res, err := client.Sync(context.Background(), events, SyncOptions{IncludePast: true, DryRun: true})
	if err != nil {
		t.Fatalf("Sync dry-run: %v", err)
	}
	if res.Added != 2 {
		t.Errorf("dry-run added=%d want 2 (reported, not written)", res.Added)
	}
	if len(backend.puts) != 0 {
		t.Errorf("dry-run issued %d PUTs, want 0", len(backend.puts))
	}
}

func TestSync_ActionLogs(t *testing.T) {
	backend := newFakeBackend()
	client, ts := newTestClient(t, backend)
	defer ts.Close()
	var logs []string
	logf := func(format string, a ...any) {
		logs = append(logs, fmt.Sprintf(format, a...))
	}

	addEvent := futureEvent("ev1")
	logs = nil
	_, err := client.Sync(context.Background(), []pipeline.Event{addEvent}, SyncOptions{IncludePast: true, Logf: logf})
	if err != nil {
		t.Fatalf("sync add: %v", err)
	}
	addLogs := strings.Join(logs, "\n")
	if len(logs) == 0 || !strings.Contains(addLogs, "wrote appointment summary=") {
		t.Fatalf("expected write log, got %v", logs)
	}
	if strings.Contains(addLogs, "uid=") {
		t.Fatalf("expected no uid in logs, got %v", logs)
	}
	if !strings.Contains(addLogs, "start=") || !strings.Contains(addLogs, "end=") {
		t.Fatalf("expected start/end in logs, got %v", logs)
	}

	updated := addEvent
	updated.Summary = "Updated"
	updated.Description = "Updated description"
	updated.DtStart = updated.DtStart.Add(26 * time.Hour)
	updated.DtEnd = updated.DtEnd.Add(26 * time.Hour)
	logs = nil
	_, err = client.Sync(context.Background(), []pipeline.Event{updated}, SyncOptions{IncludePast: true, Logf: logf})
	if err != nil {
		t.Fatalf("sync update: %v", err)
	}
	updateLogs := strings.Join(logs, "\n")
	if len(logs) == 0 || !strings.Contains(updateLogs, "changed appointment changes=") {
		t.Fatalf("expected change log, got %v", logs)
	}
	if !strings.Contains(updateLogs, "changes=") {
		t.Fatalf("expected metadata change summary in logs, got %v", logs)
	}
	if !strings.Contains(updateLogs, "changes_json=") {
		t.Fatalf("expected metadata change json in logs, got %v", logs)
	}
	if !strings.Contains(updateLogs, `"field":"title"`) || !strings.Contains(updateLogs, `"field":"description"`) || !strings.Contains(updateLogs, `"field":"date"`) || !strings.Contains(updateLogs, `"field":"time"`) {
		t.Fatalf("expected title/description/date/time JSON changes in logs, got %v", logs)
	}
	if !strings.Contains(updateLogs, "title:") || !strings.Contains(updateLogs, "description:") || !strings.Contains(updateLogs, "date:") || !strings.Contains(updateLogs, "time:") {
		t.Fatalf("expected title/description/date/time changes in logs, got %v", logs)
	}
	if strings.Contains(updateLogs, "uid=") {
		t.Fatalf("expected no uid in logs, got %v", logs)
	}
	if !strings.Contains(updateLogs, "start=") || !strings.Contains(updateLogs, "end=") {
		t.Fatalf("expected start/end in logs, got %v", logs)
	}

	dryRunUpdated := updated
	dryRunUpdated.Summary = "Updated dry-run"
	logs = nil
	_, err = client.Sync(context.Background(), []pipeline.Event{dryRunUpdated}, SyncOptions{IncludePast: true, DryRun: true, Logf: logf})
	if err != nil {
		t.Fatalf("sync dry-run update: %v", err)
	}
	dryRunLogs := strings.Join(logs, "\n")
	if !strings.Contains(dryRunLogs, "would change appointment") {
		t.Fatalf("expected dry-run change log, got %v", logs)
	}
	if !strings.Contains(dryRunLogs, "changes_json=") {
		t.Fatalf("expected dry-run metadata change json in logs, got %v", logs)
	}

	logs = nil
	_, err = client.Sync(context.Background(), []pipeline.Event{futureEvent("ev2")}, SyncOptions{IncludePast: true, Logf: logf})
	if err != nil {
		t.Fatalf("sync delete: %v", err)
	}
	deleteLogs := strings.Join(logs, "\n")
	if len(logs) == 0 || !strings.Contains(deleteLogs, "deleted appointment") {
		t.Fatalf("expected delete log, got %v", logs)
	}
	if strings.Contains(deleteLogs, "uid=") {
		t.Fatalf("expected no uid in logs, got %v", logs)
	}
	if !strings.Contains(deleteLogs, "start=") || !strings.Contains(deleteLogs, "end=") {
		t.Fatalf("expected start/end in logs, got %v", logs)
	}
}
