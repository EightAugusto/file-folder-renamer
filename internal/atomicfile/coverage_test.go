package atomicfile

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"testing"
	"time"
)

type fakeTemporary struct {
	bytes.Buffer
	name                        string
	writeErr, syncErr, closeErr error
}

func (file *fakeTemporary) Name() string { return file.name }
func (file *fakeTemporary) Write(data []byte) (int, error) {
	if file.writeErr != nil {
		return 0, file.writeErr
	}
	return file.Buffer.Write(data)
}
func (file *fakeTemporary) Sync() error  { return file.syncErr }
func (file *fakeTemporary) Close() error { return file.closeErr }

type shortTemporary struct{ fakeTemporary }

func (file *shortTemporary) Write([]byte) (int, error) { return 0, nil }

func testOperations(file temporaryFile) fileOperations {
	return fileOperations{
		mkdirAll:   func(string, os.FileMode) error { return nil },
		createTemp: func(string, string) (temporaryFile, error) { return file, nil },
		rename:     func(string, string) error { return nil },
		remove:     func(string) error { return nil },
		lstat:      func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
	}
}

func TestWriteJSONEveryFailureStage(t *testing.T) {
	sentinel := errors.New("failure")
	baseFile := func() *fakeTemporary { return &fakeTemporary{name: "/tmp/atomic-fake"} }
	tests := []struct {
		name   string
		value  any
		mutate func(*fileOperations, *fakeTemporary)
	}{
		{name: "mkdir", value: struct{}{}, mutate: func(ops *fileOperations, _ *fakeTemporary) {
			ops.mkdirAll = func(string, os.FileMode) error { return sentinel }
		}},
		{name: "create", value: struct{}{}, mutate: func(ops *fileOperations, _ *fakeTemporary) {
			ops.createTemp = func(string, string) (temporaryFile, error) { return nil, sentinel }
		}},
		{name: "encode", value: make(chan int), mutate: func(*fileOperations, *fakeTemporary) {}},
		{name: "write", value: struct{}{}, mutate: func(_ *fileOperations, file *fakeTemporary) { file.writeErr = sentinel }},
		{name: "sync", value: struct{}{}, mutate: func(_ *fileOperations, file *fakeTemporary) { file.syncErr = sentinel }},
		{name: "close", value: struct{}{}, mutate: func(_ *fileOperations, file *fakeTemporary) { file.closeErr = sentinel }},
		{name: "rename", value: struct{}{}, mutate: func(ops *fileOperations, _ *fakeTemporary) {
			ops.rename = func(string, string) error { return sentinel }
		}},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			file := baseFile()
			ops := testOperations(file)
			testCase.mutate(&ops, file)
			if err := writeJSON("/tmp/value.json", testCase.value, ops); err == nil {
				t.Fatal("expected failure")
			}
		})
	}
	file := baseFile()
	file.closeErr = sentinel
	ops := testOperations(file)
	ops.remove = func(string) error { return sentinel }
	if err := writeJSON("/tmp/value.json", make(chan int), ops); err == nil {
		t.Fatal("expected joined cleanup failures")
	}
}

func TestReplaceJSONSetEveryStagingFailure(t *testing.T) {
	sentinel := errors.New("failure")
	for _, testCase := range []struct {
		name   string
		values map[string]any
		mutate func(*fileOperations, *fakeTemporary)
	}{
		{name: "mkdir", values: map[string]any{"/tmp/a": 1}, mutate: func(ops *fileOperations, _ *fakeTemporary) {
			ops.mkdirAll = func(string, os.FileMode) error { return sentinel }
		}},
		{name: "marshal", values: map[string]any{"/tmp/a": make(chan int)}, mutate: func(*fileOperations, *fakeTemporary) {}},
		{name: "create", values: map[string]any{"/tmp/a": 1}, mutate: func(ops *fileOperations, _ *fakeTemporary) {
			ops.createTemp = func(string, string) (temporaryFile, error) { return nil, sentinel }
		}},
		{name: "write", values: map[string]any{"/tmp/a": 1}, mutate: func(_ *fileOperations, file *fakeTemporary) { file.writeErr = sentinel }},
		{name: "sync", values: map[string]any{"/tmp/a": 1}, mutate: func(_ *fileOperations, file *fakeTemporary) { file.syncErr = sentinel }},
		{name: "close", values: map[string]any{"/tmp/a": 1}, mutate: func(_ *fileOperations, file *fakeTemporary) { file.closeErr = sentinel }},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			file := &fakeTemporary{name: "/tmp/staged"}
			ops := testOperations(file)
			testCase.mutate(&ops, file)
			if err := replaceJSONSet(testCase.values, nil, ops); err == nil {
				t.Fatal("expected failure")
			}
		})
	}
	if err := replaceJSONSet(map[string]any{"/tmp/a": 1}, nil, testOperations(&shortTemporary{fakeTemporary: fakeTemporary{name: "/tmp/staged"}})); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write: %v", err)
	}
	writeFailure := errors.New("write")
	closeFailure := errors.New("close")
	removeFailure := errors.New("remove")
	file := &fakeTemporary{name: "/tmp/staged", writeErr: writeFailure, closeErr: closeFailure}
	ops := testOperations(file)
	ops.remove = func(string) error { return removeFailure }
	err := replaceJSONSet(map[string]any{"/tmp/a": 1}, nil, ops)
	for _, expected := range []error{writeFailure, closeFailure, removeFailure} {
		if !errors.Is(err, expected) {
			t.Fatalf("cleanup error does not include %v: %v", expected, err)
		}
	}
}

func TestReplaceJSONSetBackupPromotionAndRollbackFailures(t *testing.T) {
	sentinel := errors.New("failure")
	path := "/tmp/value.json"
	newFile := func() *fakeTemporary { return &fakeTemporary{name: "/tmp/staged"} }

	file := newFile()
	ops := testOperations(file)
	ops.lstat = func(string) (os.FileInfo, error) { return nil, sentinel }
	if err := replaceJSONSet(map[string]any{path: 1}, nil, ops); err == nil {
		t.Fatal("preflight inspect accepted")
	}

	for _, stage := range []string{"backup create", "backup close", "backup remove", "backup rename", "promotion"} {
		t.Run(stage, func(t *testing.T) {
			file := newFile()
			ops := testOperations(file)
			creates := 0
			renames := 0
			ops.lstat = func(string) (os.FileInfo, error) { return fakeFileInfo{}, nil }
			ops.createTemp = func(string, string) (temporaryFile, error) {
				creates++
				if stage == "backup create" && creates == 2 {
					return nil, sentinel
				}
				backup := &fakeTemporary{name: "/tmp/backup"}
				if creates == 1 {
					return file, nil
				}
				if stage == "backup close" {
					backup.closeErr = sentinel
				}
				return backup, nil
			}
			removeCalls := 0
			ops.remove = func(string) error {
				removeCalls++
				if stage == "backup remove" && removeCalls == 1 {
					return sentinel
				}
				return nil
			}
			ops.rename = func(string, string) error {
				renames++
				if stage == "backup rename" && renames == 1 {
					return sentinel
				}
				if stage == "promotion" && renames == 2 {
					return sentinel
				}
				return nil
			}
			if err := replaceJSONSet(map[string]any{path: 1}, nil, ops); err == nil {
				t.Fatal("expected failure")
			}
		})
	}
}

func TestRollbackReportsCleanupAndRestoreFailures(t *testing.T) {
	sentinel := errors.New("failure")
	ops := testOperations(&fakeTemporary{})
	ops.remove = func(string) error { return sentinel }
	ops.rename = func(string, string) error { return sentinel }
	err := rollbackSet([]stagedFile{{path: "/value", promoted: true, backup: "/backup"}}, sentinel, ops)
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok || len(joined.Unwrap()) < 3 {
		t.Fatal("expected joined errors")
	}
}

func TestRollbackPreservesBackupWhenRestoreFails(t *testing.T) {
	sentinel := errors.New("failure")
	file := &fakeTemporary{name: "/tmp/staged"}
	ops := testOperations(file)
	ops.lstat = func(string) (os.FileInfo, error) { return fakeFileInfo{}, nil }
	creates := 0
	ops.createTemp = func(string, string) (temporaryFile, error) {
		creates++
		if creates == 1 {
			return file, nil
		}
		return &fakeTemporary{name: "/tmp/backup"}, nil
	}
	renames := 0
	ops.rename = func(string, string) error {
		renames++
		if renames >= 2 {
			return sentinel
		}
		return nil
	}
	removedBackup := false
	ops.remove = func(path string) error {
		if path == "/tmp/backup" && renames > 0 {
			removedBackup = true
		}
		return nil
	}
	if err := replaceJSONSet(map[string]any{"/tmp/value.json": 1}, nil, ops); !errors.Is(err, sentinel) {
		t.Fatalf("promotion and restoration failure: %v", err)
	}
	if removedBackup {
		t.Fatal("failed restoration deleted the only backup")
	}
}

func TestReplaceJSONSetRollsBackPairDeletion(t *testing.T) {
	sentinel := errors.New("second backup")
	ops := testOperations(&fakeTemporary{})
	ops.lstat = func(string) (os.FileInfo, error) { return fakeFileInfo{}, nil }
	backup := 0
	ops.createTemp = func(string, string) (temporaryFile, error) {
		backup++
		return &fakeTemporary{name: fmt.Sprintf("/tmp/backup-%d", backup)}, nil
	}
	var renames []string
	ops.rename = func(from, to string) error {
		renames = append(renames, from+" -> "+to)
		if len(renames) == 2 {
			return sentinel
		}
		return nil
	}
	err := replaceJSONSet(nil, []string{"/tmp/pattern.json", "/tmp/pattern.tests.json"}, ops)
	if !errors.Is(err, sentinel) {
		t.Fatalf("pair deletion failure: %v", err)
	}
	want := []string{
		"/tmp/pattern.json -> /tmp/backup-1",
		"/tmp/pattern.tests.json -> /tmp/backup-2",
		"/tmp/backup-1 -> /tmp/pattern.json",
	}
	if !reflect.DeepEqual(renames, want) {
		t.Fatalf("pair deletion was not rolled back: got %v, want %v", renames, want)
	}
}

func TestReplaceJSONSetObsoleteAndLateInspectionBranches(t *testing.T) {
	path := "/tmp/value.json"
	file := &fakeTemporary{name: "/tmp/staged"}
	ops := testOperations(file)
	if err := replaceJSONSet(map[string]any{path: 1}, []string{path, "/tmp/obsolete.json"}, ops); err != nil {
		t.Fatalf("obsolete transaction: %v", err)
	}

	sentinel := errors.New("late inspect")
	file = &fakeTemporary{name: "/tmp/staged"}
	ops = testOperations(file)
	checks := 0
	ops.lstat = func(string) (os.FileInfo, error) {
		checks++
		if checks > 1 {
			return nil, sentinel
		}
		return nil, os.ErrNotExist
	}
	if err := replaceJSONSet(map[string]any{path: 1}, nil, ops); !errors.Is(err, sentinel) {
		t.Fatalf("late inspection: %v", err)
	}

	file = &fakeTemporary{name: "/tmp/staged"}
	ops = testOperations(file)
	ops.lstat = func(string) (os.FileInfo, error) { return nil, sentinel }
	ops.remove = func(string) error { return errors.New("cleanup") }
	if err := replaceJSONSet(map[string]any{path: 1}, nil, ops); err == nil {
		t.Fatal("expected inspection and cleanup errors")
	}
}

func TestReplaceJSONSetDoesNotReportPostCommitCleanupFailure(t *testing.T) {
	path := "/tmp/value.json"
	file := &fakeTemporary{name: "/tmp/staged"}
	ops := testOperations(file)
	ops.lstat = func(string) (os.FileInfo, error) { return fakeFileInfo{}, nil }
	creates := 0
	ops.createTemp = func(string, string) (temporaryFile, error) {
		creates++
		if creates == 1 {
			return file, nil
		}
		return &fakeTemporary{name: "/tmp/backup"}, nil
	}
	backupRemovals := 0
	ops.remove = func(path string) error {
		if path == "/tmp/backup" {
			backupRemovals++
		}
		if path == "/tmp/backup" && backupRemovals > 1 {
			return errors.New("cleanup")
		}
		return nil
	}
	if err := replaceJSONSet(map[string]any{path: 1}, nil, ops); err != nil {
		t.Fatalf("committed transaction reported cleanup failure: %v", err)
	}
}

type fakeFileInfo struct{}

func (fakeFileInfo) Name() string       { return "file" }
func (fakeFileInfo) Size() int64        { return 0 }
func (fakeFileInfo) Mode() os.FileMode  { return 0 }
func (fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (fakeFileInfo) IsDir() bool        { return false }
func (fakeFileInfo) Sys() any           { return nil }
