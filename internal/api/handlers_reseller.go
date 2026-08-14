package api

import (
	"net/http"
	"strconv"

	"github.com/mkoyazilim/aurapanel/internal/audit"
	"github.com/mkoyazilim/aurapanel/internal/auth"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

// requireReseller, kullanıcının reseller veya admin rolüne sahip olmasını zorunlu kılar.
func (s *Server) requireReseller(w http.ResponseWriter, r *http.Request) (*store.User, bool) {
	u, role, ok := s.requireAuth(w, r)
	if !ok {
		return nil, false
	}
	if role != "reseller" && role != "admin" {
		writeErr(w, http.StatusForbidden, "reseller role required")
		return nil, false
	}
	return u, true
}

// resellerIDFromPath, URL path'inden {id} değerini int64 olarak okur.
func resellerIDFromPath(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := r.PathValue("id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, http.StatusBadRequest, "geçersiz reseller id")
		return 0, false
	}
	return id, true
}

// ─── Admin tarafı ────────────────────────────────────────────────────────────

// handleResellerList GET /api/v1/resellers
func (s *Server) handleResellerList(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	resellers, err := s.deps.Store.ListResellers(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "reseller listesi alınamadı")
		return
	}
	// Parola hash'lerini temizle
	for i := range resellers {
		resellers[i].PasswordHash = ""
	}
	writeJSON(w, http.StatusOK, resellers)
}

// handleResellerCreate POST /api/v1/resellers
func (s *Server) handleResellerCreate(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Username == "" || req.Password == "" {
		writeErr(w, http.StatusBadRequest, "username ve password zorunludur")
		return
	}

	roleID, err := s.deps.Store.GetRoleIDByName(r.Context(), "reseller")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "reseller rolü bulunamadı")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "şifre şifrelenemedi")
		return
	}

	newUser := store.User{
		Username:     req.Username,
		PasswordHash: hash,
		RoleID:       roleID,
		Status:       "active",
	}
	id, err := s.deps.Store.InsertUser(r.Context(), newUser)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	s.deps.Audit.Write(r.Context(), audit.Event{
		Action: "reseller.create",
		Target: req.Username,
		User:   admin.Username,
		IP:     r.RemoteAddr,
		Result: "success",
	})

	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "status": "ok"})
}

// handleResellerDelete DELETE /api/v1/resellers/{id}
func (s *Server) handleResellerDelete(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	resellerID, ok := resellerIDFromPath(w, r)
	if !ok {
		return
	}

	if err := s.deps.Store.DeleteUser(r.Context(), resellerID); err != nil {
		writeErr(w, http.StatusInternalServerError, "reseller silinemedi: "+err.Error())
		return
	}

	s.deps.Audit.Write(r.Context(), audit.Event{
		Action: "reseller.delete",
		Target: strconv.FormatInt(resellerID, 10),
		User:   admin.Username,
		IP:     r.RemoteAddr,
		Result: "success",
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// quotaWithUsageResponse, quota ve kullanım bilgisini birleştirir.
type quotaWithUsageResponse struct {
	ResellerID    int64 `json:"reseller_id"`
	MaxSites      int   `json:"max_sites"`
	MaxDatabases  int   `json:"max_databases"`
	MaxDiskGB     int   `json:"max_disk_gb"`
	MaxBandwidthGB int  `json:"max_bandwidth_gb"`
	UsedSites     int   `json:"used_sites"`
	UsedDatabases int   `json:"used_databases"`
}

func buildQuotaResponse(resellerID int64, quota *store.ResellerQuota, usage store.ResellerUsage) quotaWithUsageResponse {
	resp := quotaWithUsageResponse{
		ResellerID:    resellerID,
		UsedSites:     usage.Sites,
		UsedDatabases: usage.Databases,
	}
	if quota != nil {
		resp.MaxSites = quota.MaxSites
		resp.MaxDatabases = quota.MaxDatabases
		resp.MaxDiskGB = quota.MaxDiskGB
		resp.MaxBandwidthGB = quota.MaxBandwidthGB
	}
	return resp
}

// handleResellerGetQuota GET /api/v1/resellers/{id}/quota
func (s *Server) handleResellerGetQuota(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	resellerID, ok := resellerIDFromPath(w, r)
	if !ok {
		return
	}

	quota, usage, err := s.deps.Reseller.GetQuotaWithUsage(r.Context(), resellerID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "kota bilgisi alınamadı: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, buildQuotaResponse(resellerID, quota, usage))
}

// handleResellerSetQuota PUT /api/v1/resellers/{id}/quota
func (s *Server) handleResellerSetQuota(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	resellerID, ok := resellerIDFromPath(w, r)
	if !ok {
		return
	}

	var req struct {
		MaxSites       int `json:"max_sites"`
		MaxDatabases   int `json:"max_databases"`
		MaxDiskGB      int `json:"max_disk_gb"`
		MaxBandwidthGB int `json:"max_bandwidth_gb"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.MaxSites <= 0 || req.MaxDatabases <= 0 || req.MaxDiskGB <= 0 || req.MaxBandwidthGB <= 0 {
		writeErr(w, http.StatusBadRequest, "tüm kota değerleri sıfırdan büyük olmalıdır")
		return
	}

	q := store.ResellerQuota{
		MaxSites:       req.MaxSites,
		MaxDatabases:   req.MaxDatabases,
		MaxDiskGB:      req.MaxDiskGB,
		MaxBandwidthGB: req.MaxBandwidthGB,
	}
	if err := s.deps.Reseller.AssignQuota(r.Context(), resellerID, q); err != nil {
		writeErr(w, http.StatusInternalServerError, "kota atanamadı: "+err.Error())
		return
	}

	s.deps.Audit.Write(r.Context(), audit.Event{
		Action: "reseller.set_quota",
		Target: strconv.FormatInt(resellerID, 10),
		User:   admin.Username,
		IP:     r.RemoteAddr,
		Result: "success",
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ─── Reseller tarafı ──────────────────────────────────────────────────────────

// handleResellerMe GET /api/v1/reseller/me
func (s *Server) handleResellerMe(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireReseller(w, r)
	if !ok {
		return
	}

	quota, usage, err := s.deps.Reseller.GetQuotaWithUsage(r.Context(), u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "kota bilgisi alınamadı: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, buildQuotaResponse(u.ID, quota, usage))
}

// handleResellerSites GET /api/v1/reseller/my/sites
func (s *Server) handleResellerSites(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireReseller(w, r)
	if !ok {
		return
	}

	sites, err := s.deps.Store.ListSitesByUserID(r.Context(), u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "site listesi alınamadı")
		return
	}

	writeJSON(w, http.StatusOK, sites)
}
