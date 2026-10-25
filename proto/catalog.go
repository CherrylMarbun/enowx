package proto

import "time"

// Each host publishes its projects in its synced data, catalog/<its sync
// ID>.json, so every host knows every host's projects: to attach a note
// to one elsewhere, or to find the same repository cloned on another.

// Catalog is a host's projects as it published them.
type Catalog struct {
	Host     string           `json:"host"` // SyncManifest.ID
	Name     string           `json:"name"` // its host name
	Updated  time.Time        `json:"updated"`
	Projects []CatalogProject `json:"projects"`
}

// CatalogProject is a project in a host's catalog; Remote is its Git
// origin's URL, which names it on every host it's cloned on.
type CatalogProject struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Remote string `json:"remote,omitempty"`
	Group  string `json:"group,omitempty"`
}

// MethodCatalogList lists every host's catalog this host has: its own
// and those synced to it.
const MethodCatalogList = "catalog.list" // → []Catalog
