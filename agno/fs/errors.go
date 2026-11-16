package fs

import (
	"errors"
	"fmt"
)

// ErrFileSystem is the root of every FileSystem error (errors.Is works on all).
var ErrFileSystem = errors.New("filesystem error")

// ErrNotFound is returned when a file does not exist (Python FileNotFoundError).
var ErrNotFound = fmt.Errorf("%w: file not found", ErrFileSystem)

// ErrExists is returned when a file exists and overwrite is false (Python FileExistsError).
var ErrExists = fmt.Errorf("%w: file exists", ErrFileSystem)

// InvalidPathError is returned for an invalid path, namespace, directory or record.
type InvalidPathError struct{ Msg string }

func (e *InvalidPathError) Error() string { return e.Msg }
func (e *InvalidPathError) Unwrap() error { return ErrFileSystem }

func invalidPath(original, reason string) error {
	return &InvalidPathError{Msg: fmt.Sprintf("invalid path %q: %s. Use relative paths like notes/topic.md.", original, reason)}
}

// QuotaExceededError is returned when a write would exceed a size cap.
// Scope is "file" or "namespace"; Current and Limit are byte counts.
type QuotaExceededError struct {
	Msg     string
	Scope   string
	Current int
	Limit   int
}

func (e *QuotaExceededError) Error() string { return e.Msg }
func (e *QuotaExceededError) Unwrap() error { return ErrFileSystem }

// VersionConflictError is returned when a compare-and-swap write finds another
// version. Actual is nil when the file does not exist.
type VersionConflictError struct {
	Expected int
	Actual   *int
}

func (e *VersionConflictError) Error() string {
	if e.Actual == nil {
		return fmt.Sprintf("version conflict: expected %d, file does not exist", e.Expected)
	}
	return fmt.Sprintf("version conflict: expected %d, found %d", e.Expected, *e.Actual)
}
func (e *VersionConflictError) Unwrap() error { return ErrFileSystem }

// UnsupportedOperationError is returned by a backend that does not support an operation.
type UnsupportedOperationError struct {
	Operation string
	Backend   string
	Msg       string
}

func (e *UnsupportedOperationError) Error() string { return e.Msg }
func (e *UnsupportedOperationError) Unwrap() error { return ErrFileSystem }
