// Package gamedata embeds client-side config tables (converted from tpl_*.lua
// to JSON) and exposes typed accessors used by the httpapi and ws packages.
//
// All data is loaded once at startup via Load (or lazily on first Get). The
// embedded JSON files live in ./data and are produced by
// d:\GamePS\Poker Fate\temp\lua_to_json.py.
package gamedata

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
)

//go:embed data/*.json
var dataFS embed.FS

// TableMeta records the source file, format and row count of a table.
type TableMeta struct {
	Source string `json:"source"`
	Format string `json:"format"` // "keys_bodys" or "dict"
	Count  int    `json:"count"`
}

// Table is the unified JSON shape produced by lua_to_json.py.
// For keys_bodys tables, List and Map are populated.
// For dict tables, Dict is populated.
type Table struct {
	Meta TableMeta                    `json:"_meta"`
	List []map[string]interface{}     `json:"list,omitempty"`
	Map  map[string]map[string]interface{} `json:"map,omitempty"`
	Dict map[string]interface{}       `json:"dict,omitempty"`
}

var (
	once    sync.Once
	tables  = make(map[string]*Table)
	loadErr error
)

// Load parses every embedded JSON file into memory. It is goroutine-safe and
// idempotent: subsequent calls are no-ops. The first error encountered is
// sticky and returned on every subsequent call.
func Load() error {
	once.Do(func() {
		entries, err := dataFS.ReadDir("data")
		if err != nil {
			loadErr = fmt.Errorf("gamedata: read embed dir: %w", err)
			return
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if len(name) < 6 || name[len(name)-5:] != ".json" {
				continue
			}
			data, err := dataFS.ReadFile("data/" + name)
			if err != nil {
				loadErr = fmt.Errorf("gamedata: read %s: %w", name, err)
				return
			}
			var t Table
			if err := json.Unmarshal(data, &t); err != nil {
				loadErr = fmt.Errorf("gamedata: parse %s: %w", name, err)
				return
			}
			tables[name[:len(name)-5]] = &t
		}
	})
	return loadErr
}

// MustLoad is Load but panics on error. Use at startup.
func MustLoad() {
	if err := Load(); err != nil {
		panic(err)
	}
}

// Get returns the named table (without .json extension) or nil if missing.
// Triggers Load on first call.
func Get(name string) *Table {
	if err := Load(); err != nil {
		return nil
	}
	return tables[name]
}

// MustGet is Get but panics if the table is missing. Use only for tables the
// server cannot run without.
func MustGet(name string) *Table {
	if err := Load(); err != nil {
		panic(err)
	}
	t, ok := tables[name]
	if !ok {
		panic("gamedata: table not found: " + name)
	}
	return t
}

// TableNames returns the sorted list of all loaded table names.
func TableNames() []string {
	if err := Load(); err != nil {
		return nil
	}
	out := make([]string, 0, len(tables))
	for name := range tables {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// --- helpers shared by accessors ---

// asInt32 extracts an int32 from a JSON-decoded value (number or float64).
// Returns ok=false if the value is not numeric.
func asInt32(v interface{}) (int32, bool) {
	switch n := v.(type) {
	case float64:
		return int32(n), true
	case int:
		return int32(n), true
	case int32:
		return n, true
	case int64:
		return int32(n), true
	}
	return 0, false
}

// asString extracts a string from a JSON-decoded value.
func asString(v interface{}) (string, bool) {
	if s, ok := v.(string); ok {
		return s, true
	}
	return "", false
}
