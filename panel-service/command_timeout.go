package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	defaultCommandTimeoutSeconds = 12
	minCommandTimeoutSeconds     = 2
	maxCommandTimeoutSeconds     = 120
)

func configuredCommandTimeout() time.Duration {
	raw := strings.TrimSpace(envOr("AURAPANEL_COMMAND_TIMEOUT_SECONDS", strconv.Itoa(defaultCommandTimeoutSeconds)))
	value, err := strconv.Atoi(raw)
	if err != nil {
		value = defaultCommandTimeoutSeconds
	}
	if value < minCommandTimeoutSeconds {
		value = minCommandTimeoutSeconds
	}
	if value > maxCommandTimeoutSeconds {
		value = maxCommandTimeoutSeconds
	}
	return time.Duration(value) * time.Second
}

func runCommandWithTimeout(timeout time.Duration, command string, args ...string) error {
	_, err := runCommandCombinedOutputWithTimeout(timeout, command, args...)
	return err
}

func runCommandOutput(command string, args ...string) ([]byte, error) {
	return runCommandOutputWithTimeout(configuredCommandTimeout(), command, args...)
}

func runCommandOutputWithTimeout(timeout time.Duration, command string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, command, args...)
	output, err := cmd.Output()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return output, fmt.Errorf("%s timed out after %s", command, timeout)
	}
	return output, err
}

func runCommandCombinedOutput(command string, args ...string) ([]byte, error) {
	return runCommandCombinedOutputWithTimeout(configuredCommandTimeout(), command, args...)
}

func runCommandCombinedOutputWithTimeout(timeout time.Duration, command string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, command, args...)
	output, err := cmd.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return output, fmt.Errorf("%s timed out after %s", command, timeout)
	}
	return output, err
}
