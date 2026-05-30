package parser

import (
	"fmt"
	"path/filepath"
)

// FileParser parses the raw bytes of a configuration file and writes the
// result into target. The meaning of target depends on the parser Kind —
// use Lookup to retrieve both.
type FileParser func(data []byte, target any) error

// Kind controls how Load routes the parsed result.
type Kind uint8

const (
	// KindStruct parsers unmarshal directly into the configuration struct
	// (e.g. YAML). Load passes the struct pointer as target.
	KindStruct Kind = iota
	// KindKV parsers produce a flat key-value map (e.g. dotenv). Load passes
	// a *map[string]string as target; the loader uses the map as a fallback
	// in env-var lookups so real env vars always win.
	KindKV
)

type entry struct {
	parser FileParser
	kind   Kind
}

var registry = map[string]entry{
	".yml":  {parser: YAML, kind: KindStruct},
	".yaml": {parser: YAML, kind: KindStruct},
	".env":  {parser: DotEnv, kind: KindKV},
}

// Lookup returns the FileParser and Kind registered for the extension of path.
// Returns an error if the extension is not supported.
func Lookup(path string) (FileParser, Kind, error) {
	e, ok := registry[filepath.Ext(path)]
	if !ok {
		return nil, 0, fmt.Errorf("unsupported file format: %q", filepath.Ext(path))
	}
	return e.parser, e.kind, nil
}
