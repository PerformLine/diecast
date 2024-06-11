package diecast

import "github.com/PerformLine/go-performline-stdlib/build"

const ApplicationName = `diecast`
const ApplicationSummary = `a standalone site templating engine that consumes REST services and renders static HTML output in realtime`

var ApplicationVersion = build.Version
var DiecastUserAgentString = `diecast/` + ApplicationVersion
