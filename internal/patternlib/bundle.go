package patternlib

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/eightaugusto/file-folder-renamer/internal/pattern"
)

type Bundle struct {
	Pattern  string         `json:"pattern"`
	Files    bool           `json:"files"`
	Folders  bool           `json:"folders"`
	Patterns []pattern.Spec `json:"patterns,omitempty"`
}

func DecodeBundle(reader io.Reader) (Bundle, error) {
	var configuration Bundle
	if err := json.NewDecoder(reader).Decode(&configuration); err != nil {
		return Bundle{}, err
	}
	if err := configuration.Validate(); err != nil {
		return Bundle{}, err
	}
	return configuration, nil
}

func EncodeBundle(writer io.Writer, configuration Bundle) error {
	if err := configuration.Validate(); err != nil {
		return err
	}
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(configuration)
}

func (c Bundle) Validate() error {
	if !c.Files && !c.Folders {
		return fmt.Errorf("At least one of files or folders must be enabled")
	}
	if c.Pattern == "" {
		return fmt.Errorf("Pattern must not be empty")
	}
	return nil
}
