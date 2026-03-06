// Package defaultcfg embeds the default config.yaml so it can be shared
// between cmd/make-ics and cmd/web without duplicating the file.
package defaultcfg

import _ "embed"

//go:embed config.yaml
var DefaultConfig []byte
