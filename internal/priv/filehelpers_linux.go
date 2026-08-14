//go:build linux

package priv

import "path/filepath"

// filepathEval, worker'ın symlink çözümlemesi.
func filepathEval(p string) (string, error) { return filepath.EvalSymlinks(p) }
