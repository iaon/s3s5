package version

import (
	"fmt"
	"runtime/debug"
)

var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
	Dirty   bool   `json:"dirty"`
}

func Get() Info {
	info := Info{Version: Version, Commit: Commit, Date: Date}
	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range buildInfo.Settings {
			switch setting.Key {
			case "vcs.revision":
				if info.Commit == "unknown" && setting.Value != "" {
					info.Commit = setting.Value
				}
			case "vcs.time":
				if info.Date == "unknown" && setting.Value != "" {
					info.Date = setting.Value
				}
			case "vcs.modified":
				info.Dirty = setting.Value == "true"
			}
		}
	}
	return info
}

func String() string {
	info := Get()
	dirty := ""
	if info.Dirty {
		dirty = " dirty"
	}
	return fmt.Sprintf("%s commit=%s date=%s%s", info.Version, info.Commit, info.Date, dirty)
}
