//go:build windows

package main

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// replaceFile atomically replaces path with tmp on Windows. os.Rename cannot
// overwrite an existing destination there, which breaks the tab/endpoint state
// files after their first write.
func replaceFile(tmp, path string) error {
	from, err := windows.UTF16PtrFromString(tmp)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	if err := windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH); err != nil {
		return fmt.Errorf("replace %s with %s: %w", path, tmp, err)
	}
	return nil
}
