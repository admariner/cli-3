package cli

import _ "embed"

// AgentGuide is the embedded agent guide shipped with the pscale binary.
//
//go:embed AGENTS.md
var AgentGuide string
