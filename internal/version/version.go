// Package version exposes build metadata to the desktop UI.
package version

import (
	"runtime/debug"
	"strings"
)

// Release is populated from FyneApp.toml by the release build.
var Release string

type Info struct {
	Version  string
	Revision string
	Modified bool
}

func Current() Info {
	build, _ := debug.ReadBuildInfo()
	return resolve(Release, build)
}

func Display() string {
	return display(Current())
}

func display(info Info) string {
	if info.Version != "development" {
		return info.Version
	}
	if info.Revision == "" {
		return info.Version
	}
	value := info.Version + "+" + info.Revision
	if info.Modified {
		value += "-dirty"
	}
	return value
}

func resolve(release string, build *debug.BuildInfo) Info {
	if release != "" {
		return Info{Version: strings.TrimPrefix(release, "v")}
	}
	info := Info{Version: "development"}
	if build == nil {
		return info
	}
	if build.Main.Version != "" && build.Main.Version != "(devel)" {
		info.Version = strings.TrimPrefix(build.Main.Version, "v")
	}
	for _, setting := range build.Settings {
		switch setting.Key {
		case "vcs.revision":
			info.Revision = setting.Value
			if len(info.Revision) > 12 {
				info.Revision = info.Revision[:12]
			}
		case "vcs.modified":
			info.Modified = setting.Value == "true"
		}
	}
	return info
}
