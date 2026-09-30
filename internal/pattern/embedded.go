package pattern

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"sort"

	"github.com/eightaugusto/file-folder-renamer/internal/rename"
)

// embeddedPatternFiles contains every built-in pattern compiled into the app.
//
//go:embed assets/pattern_*.json
var embeddedPatternFiles embed.FS

//go:embed assets/test_pattern_*.json
var embeddedPatternTestFiles embed.FS

type embeddedTestSuite struct {
	Pattern string             `json:"pattern"`
	Cases   []embeddedTestCase `json:"cases"`
}

type embeddedTestCase struct {
	Input             string `json:"input"`
	ExpectedName      string `json:"expected_name"`
	ExpectedExtension string `json:"expected_extension"`
}

type assetFS interface {
	fs.ReadDirFS
	fs.ReadFileFS
}

func EmbeddedSpecs() ([]Spec, error) {
	return embeddedSpecs(embeddedPatternFiles, embeddedPatternTestFiles)
}

func embeddedSpecs(patternFiles assetFS, testFiles fs.ReadFileFS) ([]Spec, error) {
	entries, err := patternFiles.ReadDir("assets")
	if err != nil {
		return nil, fmt.Errorf("read embedded pattern assets: %w", err)
	}

	fileNames := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		fileNames = append(fileNames, entry.Name())
	}
	sort.Strings(fileNames)

	specs := make([]Spec, 0, len(fileNames))
	for _, fileName := range fileNames {
		assetPath := "assets/" + fileName
		data, err := patternFiles.ReadFile(assetPath)
		if err != nil {
			return nil, fmt.Errorf("read embedded pattern %q: %w", assetPath, err)
		}

		spec, err := parseEmbeddedSpec(assetPath, data)
		if err != nil {
			return nil, err
		}
		if err := attachEmbeddedTestsFrom(testFiles, &spec, "assets/test_"+fileName); err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}

	return specs, nil
}

func attachEmbeddedTestsFrom(testFiles fs.ReadFileFS, spec *Spec, assetPath string) error {
	data, err := testFiles.ReadFile(assetPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read embedded pattern tests %q: %w", assetPath, err)
	}
	var suite embeddedTestSuite
	if err := json.Unmarshal(data, &suite); err != nil {
		return fmt.Errorf("parse embedded pattern tests %q: %w", assetPath, err)
	}
	if suite.Pattern != spec.Name {
		return fmt.Errorf("embedded pattern tests %q target %q, want %q", assetPath, suite.Pattern, spec.Name)
	}
	spec.Tests = make([]TestCase, len(suite.Cases))
	for index, testCase := range suite.Cases {
		spec.Tests[index] = TestCase{
			Input: testCase.Input, Kind: rename.NodeKindFile,
			Expected: testCase.ExpectedName + testCase.ExpectedExtension,
		}
	}
	return nil
}

func parseEmbeddedSpec(assetPath string, data []byte) (Spec, error) {
	var spec Spec
	if err := json.Unmarshal(data, &spec); err != nil {
		return Spec{}, fmt.Errorf("parse embedded pattern %q: %w", assetPath, err)
	}
	return spec, nil
}
