package api

import (
	"net/http"

	"github.com/mkoyazilim/aurapanel/internal/auth"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

func (s *Server) handleNodeAppsList(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	siteID := r.PathValue("id")

	apps, err := s.deps.Store.ListNodeApps(r.Context(), siteID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, apps)
}

func (s *Server) handleNodeAppCreate(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	siteID := r.PathValue("id")

	var req struct {
		AppName       string `json:"app_name"`
		AppPath       string `json:"app_path"`
		StartupScript string `json:"startup_script"`
		Port          int    `json:"port"`
		NodeVersion   string `json:"node_version"`
		EnvVars       string `json:"env_vars"`
	}
	if !decodeBody(w, r, &req) {
		return
	}

	if req.Port < 1024 || req.Port > 65535 {
		writeErr(w, http.StatusBadRequest, "Invalid port number")
		return
	}

	app := &store.NodeApp{
		ID:            auth.NewRequestID(),
		SiteID:        siteID,
		AppName:       req.AppName,
		AppPath:       req.AppPath,
		StartupScript: req.StartupScript,
		Port:          req.Port,
		NodeVersion:   req.NodeVersion,
		EnvVars:       req.EnvVars,
		Status:        "active",
	}
	if app.EnvVars == "" {
		app.EnvVars = "{}"
	}

	if err := s.deps.Store.InsertNodeApp(r.Context(), app); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := s.deps.Nodejs.DeployService(r.Context(), app); err != nil {
		s.deps.Store.UpdateNodeAppStatus(r.Context(), app.ID, "failed")
		writeErr(w, http.StatusInternalServerError, "Failed to deploy systemd service: "+err.Error())
		return
	}
	
	// OLS'e yansıtma (Vhost regenerate işlemi `deps.OLS.Apply` vs. normalde Job üzerinden tetiklenir)
	// Şu an API tarafında sadece DB yazıyoruz, Panel daha sonra drift algılayıp uygulayabilir
	// veya burada tetikleyebiliriz. Zaman kazanmak için drift'e bırakıyoruz (ya da apply çağırıyoruz).

	writeJSON(w, http.StatusOK, app)
}

func (s *Server) handleNodeAppDelete(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	siteID := r.PathValue("id")
	appID := r.PathValue("appId")

	if err := s.deps.Nodejs.RemoveService(r.Context(), siteID, appID); err != nil {
		// Log warning, continue deleting from DB
	}

	if err := s.deps.Store.DeleteNodeApp(r.Context(), appID); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleNodeAppRestart(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	// Restart systemd service
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Not fully implemented"})
}
