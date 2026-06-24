//go:build !windows

package main

import "os"

// replaceFile atomically replaces path with tmp on platforms where os.Rename
// supports replacing an existing destination. Windows has different semantics;
// see file_replace_windows.go.
func replaceFile(tmp, path string) error {
	return os.Rename(tmp, path)
}
