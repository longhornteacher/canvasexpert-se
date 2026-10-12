package mcpserver

import embedded "canvasexpertse"

// These values identify the adapter loaded in memory, independently of the
// Engine executable subsequently started by command tools.
var Version = embedded.ReleaseVersion()
var Revision = "development"
var BuiltAt = "unrecorded"
var BuildKind = "development"

func AdapterVersion() string { return Version + "+" + Revision + "." + BuiltAt + "." + BuildKind }
