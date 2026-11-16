package fs

import "fmt"

func sprintf(format string, args ...interface{}) string { return fmt.Sprintf(format, args...) }

// wrapf wraps a sentinel with the offending path: "filesystem error: file not found: a.md".
func wrapf(sentinel error, path string) error { return fmt.Errorf("%w: %s", sentinel, path) }

func intPtr(v int) *int { return &v }

func int64Ptr(v int64) *int64 { return &v }
