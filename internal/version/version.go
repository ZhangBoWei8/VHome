// Package version carries the build identity of the running binary. The values
// are set at link time by the Docker build:
//
//	-ldflags "-X vhome/internal/version.Commit=$GIT_COMMIT"
//
// The deploy script compares Commit against the tag it just rolled out to tell
// whether the new containers actually took over, so this is what makes an
// automatic rollback possible.
package version

var (
	// Commit is the git SHA the binary was built from.
	Commit = "dev"

	// BuildTime is an RFC3339 timestamp of the build.
	BuildTime = ""
)
