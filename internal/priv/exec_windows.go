//go:build windows

package priv

import (
	"context"
	"errors"
)

// Windows'ta priv helper desteklenmez: geliştirme yalnızca
// doğrulama/planlama mantığını (ops.go) test etmek içindir.

func requireRoot() error {
	return errors.New("aurapanel-priv yalnızca Linux'ta çalışır")
}

func executePlan(ctx context.Context, p *plan) error {
	return errors.New("plan yürütme yalnızca Linux'ta desteklenir")
}
