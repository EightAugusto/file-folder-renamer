package patternlib

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/eightaugusto/file-folder-renamer/internal/atomicfile"
	"github.com/eightaugusto/file-folder-renamer/internal/pattern"
)

var ErrReadOnly = errors.New("built-in patterns are read-only")

const fileVersion = 1

type Entry struct {
	Spec    pattern.Spec
	BuiltIn bool
}

type ConflictAction string

const (
	ConflictReplace ConflictAction = "replace"
	ConflictKeep    ConflictAction = "keep"
	ConflictCopy    ConflictAction = "copy"
)

type testFile struct {
	Version int                `json:"version"`
	Pattern string             `json:"pattern"`
	Cases   []pattern.TestCase `json:"cases"`
}

type definitionFile struct {
	Version int                `json:"version"`
	Name    string             `json:"name"`
	Rules   []pattern.RuleSpec `json:"rules"`
}

type Store struct {
	mu          sync.RWMutex
	directory   string
	builtins    map[string]pattern.Spec
	custom      map[string]pattern.Spec
	customFiles map[string]string
}

// Open creates the custom pattern directory and loads editable user pattern
// and test JSON files. Built-ins remain compiled into the binary only.
func Open(directory string) (*Store, error) {
	return openStore(directory, pattern.EmbeddedSpecs, os.MkdirAll, os.ReadDir)
}

func openStore(directory string, loadEmbedded func() ([]pattern.Spec, error), mkdirAll func(string, os.FileMode) error, readDir func(string) ([]os.DirEntry, error)) (*Store, error) {
	embedded, err := loadEmbedded()
	if err != nil {
		return nil, err
	}
	store := &Store{
		directory: directory, builtins: make(map[string]pattern.Spec),
		custom: make(map[string]pattern.Spec), customFiles: make(map[string]string),
	}
	for _, spec := range embedded {
		store.builtins[key(spec.Name)] = cloneSpec(spec)
	}
	if err := mkdirAll(directory, 0o755); err != nil {
		return store, fmt.Errorf("create pattern directory %q: %w", directory, err)
	}

	// Load valid custom definitions while retaining built-ins if one file is bad.
	var loadErrors []error
	entries, err := readDir(directory)
	if err != nil {
		return store, errors.Join(append(loadErrors, err)...)
	}
	for _, directoryEntry := range entries {
		name := directoryEntry.Name()
		if directoryEntry.IsDir() || !strings.HasSuffix(strings.ToLower(name), ".json") || strings.HasSuffix(strings.ToLower(name), ".tests.json") {
			continue
		}
		definition, readErr := readPatternFile(filepath.Join(directory, name))
		if readErr != nil {
			loadErrors = append(loadErrors, readErr)
			continue
		}
		normalized := key(definition.Name)
		if _, builtIn := store.builtins[normalized]; builtIn {
			continue
		}
		tests, testErr := readTestFile(testPath(filepath.Join(directory, name)), definition.Name)
		if testErr != nil && !errors.Is(testErr, os.ErrNotExist) {
			loadErrors = append(loadErrors, testErr)
			continue
		}
		definition.Tests = tests
		if err := validateSpec(definition); err != nil {
			loadErrors = append(loadErrors, fmt.Errorf("invalid saved pattern %q: %w", definition.Name, err))
			continue
		}
		store.custom[normalized] = cloneSpec(definition)
		store.customFiles[normalized] = name
	}
	return store, errors.Join(loadErrors...)
}

func (store *Store) Entries() []Entry {
	store.mu.RLock()
	defer store.mu.RUnlock()
	entries := make([]Entry, 0, len(store.builtins)+len(store.custom))
	for _, builtin := range store.builtins {
		entries = append(entries, Entry{Spec: cloneSpec(builtin), BuiltIn: true})
	}
	for _, custom := range store.custom {
		entries = append(entries, Entry{Spec: cloneSpec(custom)})
	}
	sort.Slice(entries, func(i, j int) bool { return key(entries[i].Spec.Name) < key(entries[j].Spec.Name) })
	return entries
}

func (store *Store) CustomSpecs() []pattern.Spec {
	store.mu.RLock()
	defer store.mu.RUnlock()
	specs := make([]pattern.Spec, 0, len(store.custom))
	for _, spec := range store.custom {
		specs = append(specs, cloneSpec(spec))
	}
	sort.Slice(specs, func(i, j int) bool { return key(specs[i].Name) < key(specs[j].Name) })
	return specs
}

func (store *Store) Lookup(name string) (Entry, error) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	normalized := key(name)
	if builtin, found := store.builtins[normalized]; found {
		return Entry{Spec: cloneSpec(builtin), BuiltIn: true}, nil
	}
	if custom, found := store.custom[normalized]; found {
		return Entry{Spec: cloneSpec(custom)}, nil
	}
	return Entry{}, fmt.Errorf("unknown pattern %q", name)
}

func (store *Store) Save(spec pattern.Spec) error { return store.SaveAs(spec.Name, spec) }

func (store *Store) SaveAs(previousName string, spec pattern.Spec) error {
	if err := validateSpec(spec); err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.saveLocked(previousName, spec)
}

func (store *Store) saveLocked(previousName string, spec pattern.Spec) error {
	// Validate ownership and name collisions before selecting disk paths.
	newKey := key(spec.Name)
	previousKey := key(previousName)
	if _, builtIn := store.builtins[newKey]; builtIn {
		return fmt.Errorf("%w: %q", ErrReadOnly, spec.Name)
	}
	if previousKey != "" {
		if _, builtIn := store.builtins[previousKey]; builtIn {
			return fmt.Errorf("%w: %q", ErrReadOnly, previousName)
		}
	}
	if _, found := store.custom[newKey]; found && newKey != previousKey {
		return fmt.Errorf("a custom pattern named %q already exists", spec.Name)
	}

	// Replace the definition and tests together, removing old files on rename.
	fileName := store.customFiles[previousKey]
	if fileName == "" || previousKey != newKey {
		fileName = store.availableFileNameLocked(spec.Name, newKey)
	}
	newPath := filepath.Join(store.directory, fileName)
	var obsolete []string
	if previousKey != "" && previousKey != newKey {
		if oldFile := store.customFiles[previousKey]; oldFile != "" {
			oldPath := filepath.Join(store.directory, oldFile)
			obsolete = []string{oldPath, testPath(oldPath)}
		}
	}
	if err := writePatternFilesReplacing(newPath, spec, obsolete); err != nil {
		return err
	}
	// Update the in-memory index only after the file transaction succeeds.
	if previousKey != "" && previousKey != newKey {
		delete(store.custom, previousKey)
		delete(store.customFiles, previousKey)
	}
	store.custom[newKey] = cloneSpec(spec)
	store.customFiles[newKey] = fileName
	return nil
}

func (store *Store) Delete(name string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	normalized := key(name)
	if _, builtIn := store.builtins[normalized]; builtIn {
		return fmt.Errorf("%w: %q", ErrReadOnly, name)
	}
	fileName, found := store.customFiles[normalized]
	if !found {
		return fmt.Errorf("unknown custom pattern %q", name)
	}
	if err := removePatternFiles(filepath.Join(store.directory, fileName)); err != nil {
		return err
	}
	delete(store.custom, normalized)
	delete(store.customFiles, normalized)
	return nil
}

func (store *Store) Has(name string) bool {
	store.mu.RLock()
	defer store.mu.RUnlock()
	normalized := key(name)
	_, custom := store.custom[normalized]
	_, builtin := store.builtins[normalized]
	return custom || builtin
}

func (store *Store) Import(specs []pattern.Spec, decisions map[string]ConflictAction) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	// Resolve each conflict against the current store state, including patterns
	// imported earlier in this bundle.
	for _, original := range specs {
		candidate := cloneSpec(original)
		if err := validateSpec(candidate); err != nil {
			return err
		}
		name := key(candidate.Name)
		_, customExists := store.custom[name]
		_, builtinExists := store.builtins[name]
		if customExists || builtinExists {
			switch decisions[name] {
			case ConflictKeep:
				continue
			case ConflictCopy:
				candidate.Name = availableCopyName(candidate.Name, store.custom, store.builtins)
			case ConflictReplace:
				if builtinExists {
					return fmt.Errorf("%w: %q", ErrReadOnly, candidate.Name)
				}
			default:
				return fmt.Errorf("pattern %q requires an import conflict decision", candidate.Name)
			}
		}
		previousName := candidate.Name
		if customExists && decisions[name] == ConflictReplace {
			previousName = original.Name
		}
		if err := store.saveLocked(previousName, candidate); err != nil {
			return err
		}
	}
	return nil
}

func (store *Store) availableFileNameLocked(name, normalized string) string {
	base := patternFileName(name)
	used := make(map[string]struct{}, len(store.customFiles)+len(store.builtins))
	for _, fileName := range store.customFiles {
		used[strings.ToLower(fileName)] = struct{}{}
	}
	for _, spec := range store.builtins {
		used[strings.ToLower(patternFileName(spec.Name))] = struct{}{}
	}
	if _, found := used[strings.ToLower(base)]; !found {
		return base
	}
	digest := sha256.Sum256([]byte(normalized))
	fallbackBase := strings.TrimSuffix(base, ".json") + "-" + hex.EncodeToString(digest[:4])
	candidate := fallbackBase + ".json"
	for suffix := 2; ; suffix++ {
		if _, found := used[strings.ToLower(candidate)]; !found {
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d.json", fallbackBase, suffix)
	}
}

func writePatternFilesReplacing(patternPath string, spec pattern.Spec, obsolete []string) error {
	spec = spec.Clone()
	definition := definitionFile{Version: fileVersion, Name: spec.Name, Rules: spec.Rules}
	tests := testFile{Version: fileVersion, Pattern: spec.Name, Cases: spec.Tests}
	if err := atomicfile.ReplaceJSONSet(map[string]any{patternPath: definition, testPath(patternPath): tests}, obsolete); err != nil {
		return fmt.Errorf("write pattern %q and its tests: %w", spec.Name, err)
	}
	return nil
}

func readPatternFile(path string) (pattern.Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return pattern.Spec{}, err
	}
	var definition definitionFile
	if err := json.Unmarshal(data, &definition); err != nil {
		return pattern.Spec{}, fmt.Errorf("parse pattern file %q: %w", path, err)
	}
	if definition.Version != fileVersion {
		return pattern.Spec{}, fmt.Errorf("pattern file %q has unsupported version %d", path, definition.Version)
	}
	return pattern.Spec{Name: definition.Name, Rules: definition.Rules}, nil
}

func readTestFile(path, patternName string) ([]pattern.TestCase, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tests testFile
	if err := json.Unmarshal(data, &tests); err != nil {
		return nil, fmt.Errorf("parse pattern tests %q: %w", path, err)
	}
	if tests.Version != fileVersion {
		return nil, fmt.Errorf("pattern tests %q have unsupported version %d", path, tests.Version)
	}
	if tests.Pattern != patternName {
		return nil, fmt.Errorf("pattern tests %q belong to %q, want %q", path, tests.Pattern, patternName)
	}
	return tests.Cases, nil
}

func removePatternFiles(patternPath string) error {
	if err := atomicfile.ReplaceJSONSet(nil, []string{patternPath, testPath(patternPath)}); err != nil {
		return fmt.Errorf("remove pattern files for %q: %w", patternPath, err)
	}
	return nil
}

func testPath(patternPath string) string {
	return strings.TrimSuffix(patternPath, filepath.Ext(patternPath)) + ".tests.json"
}

func patternFileName(name string) string {
	var builder strings.Builder
	lastDash := false
	for _, character := range strings.ToLower(strings.TrimSpace(name)) {
		if unicode.IsLetter(character) || unicode.IsDigit(character) || character == '_' {
			builder.WriteRune(character)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	base := strings.Trim(builder.String(), "-")
	if base == "" {
		base = "pattern"
	}
	return base + ".json"
}

func validateSpec(spec pattern.Spec) error {
	if _, err := pattern.Build(spec); err != nil {
		return err
	}
	for index, result := range pattern.RunTests(spec) {
		if result.Error != "" {
			return fmt.Errorf("test %d: %s", index+1, result.Error)
		}
		if !result.Passed {
			return fmt.Errorf("test %d: got %q, want %q", index+1, result.Actual, result.Case.Expected)
		}
	}
	return nil
}

func availableCopyName(name string, custom, builtins map[string]pattern.Spec) string {
	for suffix := 2; ; suffix++ {
		candidate := fmt.Sprintf("%s copy %d", name, suffix)
		_, customFound := custom[key(candidate)]
		_, builtinFound := builtins[key(candidate)]
		if !customFound && !builtinFound {
			return candidate
		}
	}
}

func key(name string) string { return strings.ToLower(strings.TrimSpace(name)) }

func cloneSpec(spec pattern.Spec) pattern.Spec {
	return spec.Clone()
}
