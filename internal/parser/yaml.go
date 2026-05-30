package parser

import "gopkg.in/yaml.v3"

// YAML unmarshals YAML-encoded data into target via yaml.Unmarshal.
var YAML FileParser = func(data []byte, target any) error {
	return yaml.Unmarshal(data, target)
}
