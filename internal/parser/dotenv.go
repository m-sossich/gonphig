package parser

import (
	"bufio"
	"bytes"
	"strings"
)

// DotEnv parses a .env file into target, which must be a *map[string]string.
//
// Supported syntax:
//
//	KEY=value         standard form
//	export KEY=value  export prefix is stripped
//	# comment         ignored
//	(empty lines)     ignored
//
// Quotes and variable expansion are not supported.
var DotEnv FileParser = func(data []byte, target any) error {
	m := target.(*map[string]string)
	*m = make(map[string]string)

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])
		if key != "" {
			(*m)[key] = value
		}
	}
	return scanner.Err()
}
