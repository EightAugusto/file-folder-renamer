package version

import (
	"runtime/debug"
	"testing"
)

func TestResolveBuildInformation(t *testing.T) {
	build := &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}, Settings: []debug.BuildSetting{
		{Key: "vcs.revision", Value: "1234567890abcdef"},
		{Key: "vcs.modified", Value: "true"},
	}}
	info := resolve("", build)
	if info.Version != "development" || info.Revision != "1234567890ab" || !info.Modified {
		t.Fatalf("unexpected development information: %+v", info)
	}
	if info := resolve("v2.3.4", build); info.Version != "2.3.4" || info.Revision != "" || info.Modified {
		t.Fatalf("release injection did not take precedence: %+v", info)
	}
}

func TestDisplayAllMetadataForms(t *testing.T) {
	previous := Release
	t.Cleanup(func() { Release = previous })
	Release = "v1.2.3"
	if Current().Version != "1.2.3" || Display() != "1.2.3" {
		t.Fatalf("release display: %+v %q", Current(), Display())
	}
	Release = ""
	if Display() == "" {
		t.Fatal("development display is empty")
	}
	if info := resolve("", nil); info.Version != "development" || info.Revision != "" {
		t.Fatalf("nil build info: %+v", info)
	}
	if info := resolve("", &debug.BuildInfo{Main: debug.Module{Version: "v2.0.0"}}); info.Version != "2.0.0" || info.Modified {
		t.Fatalf("module version: %+v", info)
	}
	clean := &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}, Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abc"}, {Key: "vcs.modified", Value: "false"}, {Key: "other", Value: "ignored"}}}
	info := resolve("", clean)
	if info.Revision != "abc" || info.Modified {
		t.Fatalf("clean metadata: %+v", info)
	}
	if got := display(Info{Version: "development"}); got != "development" {
		t.Fatalf("plain development display: %q", got)
	}
	if got := display(Info{Version: "development", Revision: "abc", Modified: true}); got != "development+abc-dirty" {
		t.Fatalf("dirty development display: %q", got)
	}
}
