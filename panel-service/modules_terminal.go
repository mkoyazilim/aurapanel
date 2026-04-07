package main

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/websocket"
)

const terminalHeartbeatFrame = "__AURA_HEARTBEAT__"

func (s *service) handleTerminalWSRoute(w http.ResponseWriter, r *http.Request) {
	server := websocket.Server{
		Handshake: terminalWebsocketHandshake,
		Handler: websocket.Handler(func(conn *websocket.Conn) {
			defer conn.Close()
			cwd := "/home"
			buffer := strings.Builder{}
			_ = websocket.Message.Send(conn, "AuraPanel terminal ready\r\nType `help` for commands.\r\n")
			sendTerminalPrompt(conn, cwd)

			for {
				var chunk string
				if err := websocket.Message.Receive(conn, &chunk); err != nil {
					return
				}
				if chunk == terminalHeartbeatFrame {
					continue
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
		}),
	}
	server.ServeHTTP(w, r)
}

func terminalWebsocketHandshake(cfg *websocket.Config, r *http.Request) error {
	if cfg == nil || r == nil {
		return fmt.Errorf("invalid websocket handshake context")
	}
	// Keep browser-origin checks in place, but include reverse-proxy forwarded host.
	if cfg.Origin == nil {
		return nil
	}

	originHost, originPort := parseHostPortForCompare(cfg.Origin.Host)
	if originHost == "" {
		return nil
	}

	candidates := []string{
		r.Host,
		forwardedHeaderValue(r.Header.Get("X-Forwarded-Host")),
	}
	if cfg.Location != nil {
		candidates = append(candidates, cfg.Location.Host)
	}

	for _, candidate := range candidates {
		host, port := parseHostPortForCompare(candidate)
		if host == "" || host != originHost {
			continue
		}
		if originPort == "" || port == "" || originPort == port {
			return nil
		}
	}

	return fmt.Errorf("websocket origin %q does not match request host", cfg.Origin.Host)
}

func parseHostPortForCompare(raw string) (string, string) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return "", ""
	}
	if strings.Contains(value, "://") {
		parsed, err := url.Parse(value)
		if err == nil {
			value = strings.ToLower(strings.TrimSpace(parsed.Host))
		}
	}
	if value == "" {
		return "", ""
	}
	if host, port, err := net.SplitHostPort(value); err == nil {
		return strings.Trim(host, "[]"), port
	}
	value = strings.Trim(value, "[]")
	lastColon := strings.LastIndex(value, ":")
	if lastColon > 0 && lastColon+1 < len(value) {
		if _, err := strconv.Atoi(value[lastColon+1:]); err == nil {
			return value[:lastColon], value[lastColon+1:]
		}
	}
	return value, ""
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
		return "Supported commands: any shell command inside managed roots, plus cd and clear", cwd
	case "pwd":
		return cwd, cwd
	case "clear":
		return "\x1b[2J\x1b[H", cwd
	default:
		return runInteractiveShell(command, cwd)
	}
}
