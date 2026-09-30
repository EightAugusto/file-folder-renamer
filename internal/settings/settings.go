package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/eightaugusto/file-folder-renamer/internal/atomicfile"
	"github.com/eightaugusto/file-folder-renamer/internal/localization"
)

var defaultIgnoredNames = []string{
	".DS_Store",
	"._*",
	".AppleDouble",
	".LSOverride",
	".Spotlight-V100",
	".TemporaryItems",
	".Trashes",
	".fseventsd",
	"Thumbs.db",
	"ehthumbs.db",
	"Desktop.ini",
	"$RECYCLE.BIN",
	"*.stackdump",
	".directory",
	".Trash-*",
	".nfs*",
	"*~",
}

var bootstrapDefaultRoot = DefaultRoot
var bootstrapOpen = Open

type Config struct {
	Language       localization.Preference `json:"language"`
	PatternsPath   string                  `json:"patterns_path"`
	IncludeFiles   bool                    `json:"include_files"`
	IncludeFolders bool                    `json:"include_folders"`
	IgnoredNames   []string                `json:"ignored_names"`
}

type Layout struct {
	BootstrapRoot string
	PatternsRoot  string
	ConfigPath    string
	Config        Config
}

func DefaultConfig(patternsPath string) Config {
	return Config{
		Language:       localization.PreferenceSystem,
		PatternsPath:   filepath.Clean(patternsPath),
		IncludeFiles:   true,
		IncludeFolders: true,
		IgnoredNames:   append([]string(nil), defaultIgnoredNames...),
	}
}

func Bootstrap() (Layout, error) {
	return bootstrap(bootstrapDefaultRoot, bootstrapOpen)
}

func bootstrap(defaultRoot func() (string, error), open func(string) (Layout, error)) (Layout, error) {
	root, err := defaultRoot()
	if err != nil {
		return Layout{}, err
	}
	return open(root)
}

// DefaultRoot keeps application-owned settings separate from the executable
// and from Fyne's internal storage. The executable's build name determines
// the final directory so differently named builds never share settings.
func DefaultRoot() (string, error) {
	return resolveDefaultRoot(os.UserHomeDir, os.Executable)
}

func resolveDefaultRoot(userHome func() (string, error), executablePath func() (string, error)) (string, error) {
	home, err := userHome()
	if err != nil {
		return "", fmt.Errorf("locate home directory: %w", err)
	}
	executable, err := executablePath()
	if err != nil {
		return "", fmt.Errorf("locate executable: %w", err)
	}
	return defaultRootFor(home, executable), nil
}

func defaultRootFor(home, executable string) string {
	name := filepath.Base(executable)
	if strings.EqualFold(filepath.Ext(name), ".exe") {
		name = strings.TrimSuffix(name, filepath.Ext(name))
	}
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = "file-folder-renamer"
	}
	return filepath.Join(home, ".eightaugusto", name)
}

func Open(bootstrapRoot string) (Layout, error) {
	return openWithOperations(bootstrapRoot, settingsOperations{
		abs: filepath.Abs, mkdirAll: os.MkdirAll, stat: os.Stat,
		load: loadForRoot, write: atomicfile.WriteJSON,
	})
}

type settingsOperations struct {
	abs      func(string) (string, error)
	mkdirAll func(string, os.FileMode) error
	stat     func(string) (os.FileInfo, error)
	load     func(string, string) (Config, error)
	write    func(string, any) error
}

func openWithOperations(bootstrapRoot string, ops settingsOperations) (Layout, error) {
	absoluteRoot, err := ops.abs(bootstrapRoot)
	if err != nil {
		return Layout{}, fmt.Errorf("resolve resources directory: %w", err)
	}
	layout := Layout{BootstrapRoot: absoluteRoot, ConfigPath: filepath.Join(absoluteRoot, "config.json")}
	if err := ops.mkdirAll(absoluteRoot, 0o755); err != nil {
		return layout, fmt.Errorf("create resources directory %q: %w", absoluteRoot, err)
	}
	_, statErr := ops.stat(layout.ConfigPath)
	configMissing := errors.Is(statErr, os.ErrNotExist)
	if statErr != nil && !configMissing {
		return layout, fmt.Errorf("inspect application config: %w", statErr)
	}
	configuration, err := ops.load(layout.ConfigPath, absoluteRoot)
	if err != nil {
		return layout, err
	}
	if configMissing {
		if err := ops.write(layout.ConfigPath, configuration); err != nil {
			return layout, fmt.Errorf("write application config: %w", err)
		}
	}
	if err := ops.mkdirAll(configuration.PatternsPath, 0o755); err != nil {
		return layout, fmt.Errorf("create patterns directory %q: %w", configuration.PatternsPath, err)
	}
	layout.PatternsRoot = configuration.PatternsPath
	layout.Config = configuration
	return layout, nil
}

func (configuration Config) Validate() error {
	if configuration.Language != "" && !configuration.Language.Valid() {
		return fmt.Errorf("unsupported language %q", configuration.Language)
	}
	if configuration.PatternsPath == "" {
		return errors.New("patterns_path must not be empty")
	}
	if !filepath.IsAbs(configuration.PatternsPath) {
		return errors.New("patterns_path must be absolute")
	}
	if !configuration.IncludeFiles && !configuration.IncludeFolders {
		return errors.New("include_files, include_folders, or both must be enabled")
	}
	seen := make(map[string]struct{}, len(configuration.IgnoredNames))
	for _, pattern := range configuration.IgnoredNames {
		if pattern == "" {
			return errors.New("ignored_names must not contain an empty pattern")
		}
		if strings.ContainsAny(pattern, `/\`) || filepath.Base(pattern) != pattern {
			return fmt.Errorf("ignored name pattern %q must not contain a directory", pattern)
		}
		if _, err := filepath.Match(pattern, "validation-name"); err != nil {
			return fmt.Errorf("invalid ignored name pattern %q: %w", pattern, err)
		}
		normalized := strings.ToLower(pattern)
		if _, duplicate := seen[normalized]; duplicate {
			return fmt.Errorf("ignored name pattern %q is duplicated", pattern)
		}
		seen[normalized] = struct{}{}
	}
	return nil
}

func Save(path string, configuration Config) error {
	if configuration.Language == "" {
		configuration.Language = localization.PreferenceSystem
	}
	configuration.PatternsPath = filepath.Clean(configuration.PatternsPath)
	if err := configuration.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return atomicfile.WriteJSON(path, configuration)
}

// Load reads and validates an application settings file without modifying it.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read application config %q: %w", path, err)
	}
	var configuration Config
	configuration.Language = localization.PreferenceSystem
	if err := json.Unmarshal(data, &configuration); err != nil {
		return Config{}, fmt.Errorf("parse application config %q: %w", path, err)
	}
	if err := configuration.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate application config %q: %w", path, err)
	}
	configuration.PatternsPath = filepath.Clean(configuration.PatternsPath)
	return configuration, nil
}

// loadForRoot reads settings over the current defaults for this build.
func loadForRoot(path, absoluteRoot string) (Config, error) {
	configuration := DefaultConfig(filepath.Join(absoluteRoot, "patterns"))
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return configuration, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read application config: %w", err)
	}
	if err := json.Unmarshal(data, &configuration); err != nil {
		return Config{}, fmt.Errorf("parse application config %q: %w", path, err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil || fields == nil {
		return Config{}, fmt.Errorf("application config %q must be a JSON object", path)
	}
	if err := configuration.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate application config %q: %w", path, err)
	}
	configuration.PatternsPath = filepath.Clean(configuration.PatternsPath)
	return configuration, nil
}
