package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/mkoyazilim/aurapanel/internal/cron"
	"github.com/mkoyazilim/aurapanel/internal/logger"
	"github.com/mkoyazilim/aurapanel/internal/security"
)

// Deps'e yeni servisler eklendi — server.go'daki Deps struct'ı güncellenmeli.
// Bu dosya: Cron, Metrics, Security, Health, Logs handler'ları.

// --- Cron ---

func (s *Server) handleCronList(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	siteID := r.PathValue("id")
	jobs, err := s.deps.Cron.List(r.Context(), siteID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, jobs)
}

func (s *Server) handleCronCreate(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	siteID := r.PathValue("id")
	var req cron.CreateRequest
	if !decodeBody(w, r, &req) {
		return
	}
	id, err := s.deps.Cron.Create(r.Context(), siteID, req)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (s *Server) handleCronUpdate(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	siteID := r.PathValue("id")
	jobID, err := strconv.ParseInt(r.PathValue("jobid"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "geçersiz job id")
		return
	}
	var req cron.CreateRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.deps.Cron.Update(r.Context(), siteID, jobID, req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleCronDelete(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	siteID := r.PathValue("id")
	jobID, err := strconv.ParseInt(r.PathValue("jobid"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "geçersiz job id")
		return
	}
	if err := s.deps.Cron.Delete(r.Context(), siteID, jobID); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Metrics ---

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	siteID := r.PathValue("id")
	hours := 24
	if h := r.URL.Query().Get("hours"); h != "" {
		if v, err := strconv.Atoi(h); err == nil && v > 0 && v <= 168 {
			hours = v
		}
	}
	data, err := s.deps.Store.ListMetrics(r.Context(), siteID, hours)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// --- Security ---

func (s *Server) handleSecurityGet(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	siteID := r.PathValue("id")
	profile, err := s.deps.Security.GetProfile(r.Context(), siteID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"profile":  profile,
		"profiles": security.Profiles,
	})
}

func (s *Server) handleSecuritySet(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	siteID := r.PathValue("id")
	var req struct {
		Profile string `json:"profile"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.deps.Security.SetProfile(r.Context(), siteID, req.Profile); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Health ---

func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	report := s.deps.Health.Run(r.Context())
	code := http.StatusOK
	if !report.Overall {
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, report)
}

// --- Live Logs (WebSocket) ---

func (s *Server) handleLogsTail(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	siteID := r.PathValue("id")
	logFile := r.URL.Query().Get("file")
	if logFile != "access" && logFile != "error" {
		logFile = "access"
	}

	// WebSocket upgrade (stdlib hijack)
	if r.Header.Get("Upgrade") != "websocket" {
		writeErr(w, http.StatusBadRequest, "WebSocket bağlantısı gerekli")
		return
	}

	sitesRoot := s.deps.Cfg.Paths.SitesRoot
	lines := make(chan logger.TailLine, 64)

	// Context: istemci bağlantıyı keserse ctx iptal edilir.
	ctx := r.Context()

	go func() {
		_ = logger.Tail(ctx, logger.TailOptions{
			SiteID:   siteID,
			LogFile:  logFile,
			LogsRoot: sitesRoot,
		}, lines)
		close(lines)
	}()

	// Basit SSE (Server-Sent Events) — gerçek WS öncesi MVP çözüm.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	flusher, canFlush := w.(http.Flusher)
	enc := json.NewEncoder(w)

	for line := range lines {
		w.Write([]byte("data: "))
		enc.Encode(line)
		w.Write([]byte("\n"))
		if canFlush {
			flusher.Flush()
		}
	}
}
