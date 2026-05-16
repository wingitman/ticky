package version

// Commit is injected at build time with -ldflags. The development fallback is
// explicit so update checks can fall back to config/repo state.
var Commit = "dev"
