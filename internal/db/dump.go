package db

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

// MariaDBDumpEngine implements backup.DumpEngine using the mysqldump CLI.
type MariaDBDumpEngine struct {
	user     string
	password string
	socket   string
}

// NewMariaDBDumpEngine creates a new MariaDBDumpEngine.
func NewMariaDBDumpEngine(user, password, socket string) *MariaDBDumpEngine {
	return &MariaDBDumpEngine{
		user:     user,
		password: password,
		socket:   socket,
	}
}

// DumpDatabase streams a mysqldump of the specified dbName to the given io.Writer.
func (e *MariaDBDumpEngine) DumpDatabase(ctx context.Context, dbName string, w io.Writer) error {
	// e.g. mysqldump -u <user> -p<password> -S <socket> <dbName>
	args := []string{
		"-u", e.user,
		fmt.Sprintf("-p%s", e.password),
	}
	if e.socket != "" {
		args = append(args, "-S", e.socket)
	}
	args = append(args, dbName)

	cmd := exec.CommandContext(ctx, "mysqldump", args...)
	cmd.Stdout = w
	// We might want to capture stderr for debugging if it fails, but for now just run it.
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mysqldump failed: %w", err)
	}
	return nil
}
