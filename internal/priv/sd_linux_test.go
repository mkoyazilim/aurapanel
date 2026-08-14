//go:build linux

package priv

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// TestSdListenEnabledEnv: yalnızca doğru env kombinasyonunda aktivasyon algılanır.
func TestSdListenEnabledEnv(t *testing.T) {
	t.Setenv("LISTEN_FDS", "1")
	t.Setenv("LISTEN_PID", strconv.Itoa(os.Getpid()))
	if !sdListenEnabled() {
		t.Fatal("LISTEN_FDS=1 + LISTEN_PID=kendimiz → aktivasyon algılanmalı")
	}
	t.Setenv("LISTEN_PID", "99999999")
	if sdListenEnabled() {
		t.Fatal("yabancı LISTEN_PID aktivasyon sayılmamalı")
	}
	t.Setenv("LISTEN_PID", "")
	t.Setenv("LISTEN_FDS", "0")
	if sdListenEnabled() {
		t.Fatal("LISTEN_FDS=0 aktivasyon sayılmamalı")
	}
}

// TestOpenListenerFdInheritance: fd 3'e kopyalanan gerçek bir unix socket,
// systemd davranışıyla (LISTEN_FDS=1) devralınabilmeli ve bağlantı kabul etmeli.
func TestOpenListenerFdInheritance(t *testing.T) {
	path := filepath.Join(t.TempDir(), "priv.sock")
	real, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer real.Close()

	raw, err := real.(*net.UnixListener).File() // dup: systemd'nin fd 3'ü taklidi
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Dup2(int(raw.Fd()), 3); err != nil {
		t.Fatal(err)
	}
	raw.Close()

	t.Setenv("LISTEN_FDS", "1")
	t.Setenv("LISTEN_PID", strconv.Itoa(os.Getpid()))
	ln, err := openListener(path, 0)
	if err != nil {
		t.Fatalf("openListener: %v", err)
	}
	defer ln.Close()

	// Bağlantı, devralınan fd üzerinden kabul edilmeli.
	done := make(chan error, 1)
	go func() {
		c, err := net.Dial("unix", path)
		if err != nil {
			done <- err
			return
		}
		_, err = c.Write([]byte("ping"))
		c.Close()
		done <- err
	}()
	ln.(*net.UnixListener).SetDeadline(time.Now().Add(3 * time.Second))
	conn, err := ln.Accept()
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	defer conn.Close()
	buf := make([]byte, 4)
	if _, err := conn.Read(buf); err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(buf) != "ping" {
		t.Fatalf("içerik: %q", buf)
	}
	if err := <-done; err != nil {
		t.Fatalf("dial: %v", err)
	}
}

// TestOpenListenerStdinInheritance: systemd'nin StandardInput=socket modunda
// dinleyen soket fd 0'dadır (LISTEN_FDS yoktur) — openListener devralmalıdır.
func TestOpenListenerStdinInheritance(t *testing.T) {
	path := filepath.Join(t.TempDir(), "priv.sock")
	real, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer real.Close()

	raw, err := real.(*net.UnixListener).File()
	if err != nil {
		t.Fatal(err)
	}
	saved, err := syscall.Dup(0) // stdin'i geri yüklemek için yedekle
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(saved)
	if err := syscall.Dup2(int(raw.Fd()), 0); err != nil {
		t.Fatal(err)
	}
	raw.Close()
	defer syscall.Dup2(saved, 0) // test sonunda stdin'i geri ver

	ln, err := openListener(path, 0)
	if err != nil {
		t.Fatalf("openListener: %v", err)
	}
	defer ln.Close()

	done := make(chan error, 1)
	go func() {
		c, err := net.Dial("unix", path)
		if err != nil {
			done <- err
			return
		}
		_, err = c.Write([]byte("stdin"))
		c.Close()
		done <- err
	}()
	ln.(*net.UnixListener).SetDeadline(time.Now().Add(3 * time.Second))
	conn, err := ln.Accept()
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	defer conn.Close()
	buf := make([]byte, 5)
	if _, err := conn.Read(buf); err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(buf) != "stdin" {
		t.Fatalf("içerik: %q", buf)
	}
	if err := <-done; err != nil {
		t.Fatalf("dial: %v", err)
	}
}
