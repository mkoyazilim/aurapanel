package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/websocket"
)

func (s *service) handleTerminalWSRoute(w http.ResponseWriter, r *http.Request) {
	websocket.Handler(func(conn *websocket.Conn) {
		defer conn.Close()
		cwd := "/home"
		buffer := strings.Builder{}
		_ = websocket.Message.Send(conn, "AuraPanel pseudo-terminal ready\r\nType `help` for commands.\r\n")
		sendTerminalPrompt(conn, cwd)

		for {
			var chunk string
			if err := websocket.Message.Receive(conn, &chunk); err != nil {
				return
			}
			for _, ch := range chunk {
				switch ch {
				case 3:
					buffer.Reset()
					_ = websocket.Message.Send(conn, "^C\r\n")
					sendTerminalPrompt(conn, cwd)
				case '\b', 127:
					current := buffer.String()
					if current == "" {
						continue
					}
					buffer.Reset()
					buffer.WriteString(current[:len(current)-1])
					_ = websocket.Message.Send(conn, "\b \b")
				case '\r', '\n':
					command := strings.TrimSpace(buffer.String())
					buffer.Reset()
					_ = websocket.Message.Send(conn, "\r\n")
					output, nextCwd := s.executeTerminalCommand(command, cwd)
					cwd = nextCwd
					if output != "" {
						_ = websocket.Message.Send(conn, output)
						if !strings.HasSuffix(output, "\r\n") {
							_ = websocket.Message.Send(conn, "\r\n")
						}
					}
					sendTerminalPrompt(conn, cwd)
				default:
					buffer.WriteRune(ch)
					_ = websocket.Message.Send(conn, string(ch))
				}
			}
		}
	}).ServeHTTP(w, r)
}

func sendTerminalPrompt(conn *websocket.Conn, cwd string) {
	prompt := fmt.Sprintf("aura@panel:%s$ ", cwd)
	_ = websocket.Message.Send(conn, prompt)
}

func (s *service) executeTerminalCommand(command, cwd string) (string, string) {
	if command == "" {
		return "", cwd
	}
	parts := strings.Fields(command)
	switch parts[0] {
	case "help":
		return "Supported commands: help, pwd, ls, cd, whoami, date, clear, echo", cwd
	case "pwd":
		return cwd, cwd
	case "whoami":
		return "admin", cwd
	case "date":
		return time.Now().UTC().Format(time.RFC1123), cwd
	case "echo":
		return strings.TrimSpace(strings.TrimPrefix(command, "echo")), cwd
	case "clear":
		return "\x1b[2J\x1b[H", cwd
	case "ls":
		s.mu.RLock()
		defer s.mu.RUnlock()
		items := s.listVirtualEntriesLocked(cwd)
		names := make([]string, 0, len(items))
		for _, item := range items {
			name := item.Name
			if item.IsDir {
				name += "/"
			}
			names = append(names, name)
		}
		return strings.Join(names, "  "), cwd
	case "cd":
		if len(parts) < 2 {
			return "usage: cd <path>", cwd
		}
		target := normalizeVirtualPath(parts[1])
		if !strings.HasPrefix(target, "/") {
			target = normalizeVirtualPath(cwd + "/" + target)
		}
		s.mu.RLock()
		defer s.mu.RUnlock()
		item, ok := s.getVirtualFileLocked(target)
		if !ok || !item.IsDir {
			return "directory not found", cwd
		}
		return "", target
	default:
		return fmt.Sprintf("command `%s` executed in simulation mode", command), cwd
	}
}
