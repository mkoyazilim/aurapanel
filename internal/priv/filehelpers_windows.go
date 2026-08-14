//go:build windows

package priv

import "errors"

func filepathEval(p string) (string, error) {
	return "", errors.New("worker yalnızca Linux'ta")
}
