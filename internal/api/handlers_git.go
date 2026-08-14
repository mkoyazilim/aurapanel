package api

import (
	"context"
	"net/http"

	"github.com/mkoyazilim/aurapanel/internal/auth"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

func (s *Server) handleGitGet(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	siteID := r.PathValue("id")

	g, err := s.deps.Store.GetGitDeployment(r.Context(), siteID)
	if err != nil {
		if err == store.ErrNotFound {
			writeJSON(w, http.StatusOK, map[string]any{"configured": false})
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"configured":       true,
		"repo_url":         g.RepoURL,
		"branch":           g.Branch,
		"deploy_path":      g.DeployPath,
		"deploy_script":    g.DeployScript,
		"webhook_secret":   g.WebhookSecret,
		"last_deployed_at": g.LastDeployedAt,
		"status":           g.Status,
	})
}

func (s *Server) handleGitSetup(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	siteID := r.PathValue("id")

	var req struct {
		RepoURL      string `json:"repo_url"`
		Branch       string `json:"branch"`
		DeployPath   string `json:"deploy_path"`
		DeployScript string `json:"deploy_script"`
	}
	if !decodeBody(w, r, &req) {
		return
	}

	g := &store.GitDeployment{
		ID:            auth.NewRequestID(),
		SiteID:        siteID,
		RepoURL:       req.RepoURL,
		Branch:        req.Branch,
		DeployPath:    req.DeployPath,
		DeployScript:  req.DeployScript,
		WebhookSecret: auth.NewRequestID(),
		Status:        "pending",
	}

	existing, err := s.deps.Store.GetGitDeployment(r.Context(), siteID)
	if err == nil {
		g.ID = existing.ID
		g.WebhookSecret = existing.WebhookSecret
	}

	if err := s.deps.Store.UpsertGitDeployment(r.Context(), g); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "webhook_secret": g.WebhookSecret})
}

func (s *Server) handleGitDeploy(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	siteID := r.PathValue("id")

	go func() {
		_ = s.deps.Git.Deploy(context.Background(), siteID)
	}()

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": "deploying"})
}

func (s *Server) handleGitDelete(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	siteID := r.PathValue("id")

	if err := s.deps.Store.DeleteGitDeployment(r.Context(), siteID); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleGitWebhook(w http.ResponseWriter, r *http.Request) {
	secret := r.PathValue("secret")
	if secret == "" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	g, err := s.deps.Store.GetGitDeploymentBySecret(r.Context(), secret)
	if err != nil {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	go func() {
		_ = s.deps.Git.Deploy(context.Background(), g.SiteID)
	}()

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Deploy triggered"})
}
