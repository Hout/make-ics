package caldav

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	webdav "github.com/emersion/go-webdav"
	davlib "github.com/emersion/go-webdav/caldav"

	"github.com/jeroen/make-ics-go/pkg/model"
)

// Client is bound to a single CalDAV calendar collection on a remote server.
type Client struct {
	dav         *davlib.Client
	calendarURL string // path of the matched calendar collection
}

// New authenticates against the CalDAV server described by cfg (using Basic
// auth with username/password), discovers the user's principal and calendar
// home set, and returns a Client bound to the calendar whose display name
// matches cfg.CalendarDisplayName.
func New(ctx context.Context, cfg model.CalDAVConfig, username, password string) (*Client, error) {
	hc := webdav.HTTPClientWithBasicAuth(http.DefaultClient, username, password)
	dav, err := davlib.NewClient(hc, cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("caldav: init client: %w", err)
	}

	principal, err := dav.FindCurrentUserPrincipal(ctx)
	if err != nil {
		return nil, fmt.Errorf("caldav: finding principal: %w", err)
	}

	homeSet, err := dav.FindCalendarHomeSet(ctx, principal)
	if err != nil {
		return nil, fmt.Errorf("caldav: finding calendar home set: %w", err)
	}

	calendars, err := dav.FindCalendars(ctx, homeSet)
	if err != nil {
		return nil, fmt.Errorf("caldav: listing calendars: %w", err)
	}

	for _, cal := range calendars {
		if cal.Name == cfg.CalendarDisplayName {
			return &Client{dav: dav, calendarURL: cal.Path}, nil
		}
	}

	names := make([]string, 0, len(calendars))
	for _, cal := range calendars {
		if cal.Name != "" {
			names = append(names, fmt.Sprintf("%q", cal.Name))
		}
	}
	return nil, fmt.Errorf("caldav: calendar %q not found (available: %s)",
		cfg.CalendarDisplayName, strings.Join(names, ", "))
}
