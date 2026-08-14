//go:build windows

package priv

import (
	"context"
	"encoding/json"
	"errors"
)

func runFileOp(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	return nil, errors.New("file.op yalnızca Linux'ta desteklenir")
}

func WorkerMain(argv []string) int { return 1 }

func fileWorkerMain(argv []string) int {
	return 1
}
