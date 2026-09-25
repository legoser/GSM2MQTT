package version

import (
	"runtime/debug"
)

// Version and BuildTime are populated at build time via ldflags,
// extracted via runtime/debug build info, or fall back to "dev".
var (
	Version   = "dev"
	BuildTime = "unknown"
)

func init() {
	if bi, ok := debug.ReadBuildInfo(); ok {
		if Version == "dev" && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			Version = bi.Main.Version
		}
		if BuildTime == "unknown" {
			for _, s := range bi.Settings {
				if s.Key == "vcs.time" {
					BuildTime = s.Value
					break
				}
			}
		}
	}
}

// String returns the formatted application version string.
func String() string {
	return "gsm2mqtt " + Version + " (built " + BuildTime + ")"
}
