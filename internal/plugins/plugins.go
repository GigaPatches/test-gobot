// Package plugins is where gobot's feature plugins are collected. The bot loads
// them in order at startup; adding a plugin means adding one line here.
package plugins

import (
	"gobot/internal/plugin"
	"gobot/internal/plugins/calc"
)

// All is every plugin a bot registers, in load order.
var All = []plugin.Plugin{
	calc.New(),
}
