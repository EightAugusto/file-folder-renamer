package atomicfile

import (
	"io"
	"os"
)

type temporaryFile interface {
	io.Writer
	Name() string
	Sync() error
	Close() error
}

type fileOperations struct {
	mkdirAll   func(string, os.FileMode) error
	createTemp func(string, string) (temporaryFile, error)
	rename     func(string, string) error
	remove     func(string) error
	lstat      func(string) (os.FileInfo, error)
}

func productionOperations() fileOperations {
	return fileOperations{
		mkdirAll:   os.MkdirAll,
		createTemp: func(directory, pattern string) (temporaryFile, error) { return os.CreateTemp(directory, pattern) },
		rename:     os.Rename, remove: os.Remove, lstat: os.Lstat,
	}
}
