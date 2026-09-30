package pattern

import (
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/eightaugusto/file-folder-renamer/internal/rename"
	"github.com/eightaugusto/file-folder-renamer/internal/rules"
)

func TestBuildRemainingValidationBranches(t *testing.T) {
	for _, spec := range []Spec{
		{},
		{Name: "x", Rules: []RuleSpec{{Kind: rules.KindCase, Mode: rules.CaseModeLower, Positions: []int{-1}}}},
		{Name: "x", Rules: []RuleSpec{{Kind: rules.KindRemoveDiacritics}}},
		{Name: "x", Rules: []RuleSpec{{Kind: rules.Kind(99)}}},
	} {
		if _, err := Build(spec); err == nil {
			t.Fatalf("expected invalid spec to fail: %+v", spec)
		}
	}
}

func TestEmbeddedAssetFailureBranches(t *testing.T) {
	validPattern := []byte(`{"name":"example","rules":[]}`)
	validTests := []byte(`{"pattern":"example","cases":[{"input":"a","expected_name":"A","expected_extension":".txt"}]}`)
	patterns := fstest.MapFS{
		"assets/directory/placeholder": &fstest.MapFile{Data: nil},
		"assets/pattern_example.json":  &fstest.MapFile{Data: validPattern},
	}
	tests := fstest.MapFS{"assets/test_pattern_example.json": &fstest.MapFile{Data: validTests}}
	specs, err := embeddedSpecs(patterns, tests)
	if err != nil || len(specs) != 1 || len(specs[0].Tests) != 1 || specs[0].Tests[0].Kind != rename.NodeKindFile {
		t.Fatalf("embedded specs: %+v %v", specs, err)
	}
	if _, err := embeddedSpecs(fstest.MapFS{}, tests); err == nil {
		t.Fatal("missing pattern directory accepted")
	}
	if _, err := embeddedSpecs(failingAssets{MapFS: patterns, failPath: "assets/pattern_example.json"}, tests); err == nil {
		t.Fatal("pattern read failure accepted")
	}
	invalidPatterns := fstest.MapFS{"assets/pattern_bad.json": &fstest.MapFile{Data: []byte(`{`)}}
	if _, err := embeddedSpecs(invalidPatterns, fstest.MapFS{}); err == nil {
		t.Fatal("invalid pattern JSON accepted")
	}
	if _, err := embeddedSpecs(patterns, fstest.MapFS{"assets/test_pattern_example.json": &fstest.MapFile{Data: []byte(`{`)}}); err == nil {
		t.Fatal("invalid test JSON accepted")
	}
	wrongTests := fstest.MapFS{"assets/test_pattern_example.json": &fstest.MapFile{Data: []byte(`{"pattern":"other"}`)}}
	if _, err := embeddedSpecs(patterns, wrongTests); err == nil {
		t.Fatal("mismatched test suite accepted")
	}
	if _, err := embeddedSpecs(patterns, failingReadFS{}); err == nil {
		t.Fatal("test read failure accepted")
	}
	spec := Spec{Name: "example"}
	if err := attachEmbeddedTestsFrom(tests, &spec, "assets/missing.json"); err != nil {
		t.Fatalf("missing tests should be optional: %v", err)
	}
}

type failingAssets struct {
	fstest.MapFS
	failPath string
}

func (assets failingAssets) ReadFile(name string) ([]byte, error) {
	if name == assets.failPath {
		return nil, errors.New("read failure")
	}
	return assets.MapFS.ReadFile(name)
}

type failingReadFS struct{}

func (failingReadFS) Open(string) (fs.File, error) {
	return nil, &fs.PathError{Op: "open", Path: "tests", Err: fs.ErrPermission}
}

func (failingReadFS) ReadFile(string) ([]byte, error) {
	return nil, &fs.PathError{Op: "read", Path: "tests", Err: fs.ErrPermission}
}

func TestTraceRemainingBranches(t *testing.T) {
	base := Spec{Name: "trace"}
	trace, err := Trace(base, "FILE.TXT", "")
	if err != nil || trace.Kind != rename.NodeKindFile {
		t.Fatalf("default kind: %+v %v", trace, err)
	}
	if _, err := Trace(base, "x", rename.NodeKind("device")); err == nil {
		t.Fatal("unknown kind accepted")
	}
	if _, err := Trace(Spec{}, "x", rename.NodeKindFile); err == nil {
		t.Fatal("invalid pattern traced")
	}
	if trace, err := Trace(base, "folder", rename.NodeKindFolder); err != nil || trace.Output != "folder" {
		t.Fatalf("folder trace: %+v %v", trace, err)
	}
	sentinel := errors.New("rule failure")
	trace, err = traceWithBuilder(base, "folder", rename.NodeKindFolder, func(Spec) (Pattern, error) {
		return Pattern{Name: "trace", Rules: []rules.Rule{failingRule{err: sentinel}}}, nil
	})
	if !errors.Is(err, sentinel) || len(trace.Steps) != 1 || trace.Steps[0].Error == "" {
		t.Fatalf("rule failure trace: %+v %v", trace, err)
	}
	results := RunTests(Spec{Tests: []TestCase{{Input: "x", Kind: rename.NodeKind("device")}}})
	if len(results) != 1 || results[0].Error == "" {
		t.Fatalf("test error missing: %+v", results)
	}
}

type failingRule struct{ err error }

func (rule failingRule) Apply(string) (string, error) { return "", rule.err }
func (failingRule) Kind() rules.Kind                  { return rules.KindCase }

func TestSpecCloneCopiesEveryNestedSlice(t *testing.T) {
	original := Spec{Rules: []RuleSpec{{
		Positions: []int{1}, Remove: []string{"a"}, Preserve: []string{"b"}, ExcludeMatches: []string{"c"},
	}}, Tests: []TestCase{{Input: "test"}}}
	clone := original.Clone()
	clone.Rules[0].Positions[0] = 2
	clone.Rules[0].Remove[0] = "x"
	clone.Rules[0].Preserve[0] = "y"
	clone.Rules[0].ExcludeMatches[0] = "z"
	clone.Tests[0].Input = "changed"
	if original.Rules[0].Positions[0] != 1 || original.Rules[0].Remove[0] != "a" || original.Rules[0].Preserve[0] != "b" || original.Rules[0].ExcludeMatches[0] != "c" || original.Tests[0].Input != "test" {
		t.Fatal("clone shared nested storage")
	}
}
