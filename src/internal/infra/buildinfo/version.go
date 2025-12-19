// Package buildinfo provides build-time version information.
//
// Values are injected at build time via ldflags:
//
//	go build -ldflags "-X github.com/yndnr/tokmesh-go/internal/infra/buildinfo.Version=v1.0.0"
package buildinfo

// Build-time variables (set via ldflags).
var (
	// Version is the semantic version.
	Version = "dev"

	// Commit is the git commit hash.
	Commit = "unknown"

	// BuildTime is the build timestamp.
	BuildTime = "unknown"

	// GoVersion is the Go version used to build.
	GoVersion = "unknown"
)

// Info contains build information.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
	GoVersion string `json:"go_version"`
}

// Get returns the build information.
func Get() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		BuildTime: BuildTime,
		GoVersion: GoVersion,
	}
}

// String returns a formatted version string.
func String() string {
	return Version + " (" + Commit + ") built at " + BuildTime
}
