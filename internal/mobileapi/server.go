package mobileapi

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
)

type Server struct {
	auth     *ERPAuthenticator
	sessions *SessionManager
}

func NewServer(auth *ERPAuthenticator) *Server {
	return &Server{
		auth:     auth,
		sessions: NewSessionManager(),
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/v1/mobile/auth/login", s.handleLogin)
	mux.HandleFunc("/v1/mobile/auth/logout", s.handleLogout)
	mux.HandleFunc("/v1/mobile/me", s.handleMe)
	mux.HandleFunc("/v1/mobile/profile", s.handleProfile)
	mux.HandleFunc("/v1/mobile/profile/avatar", s.handleProfileAvatar)
	mux.HandleFunc("/v1/mobile/supplier/history", s.handleSupplierHistory)
	mux.HandleFunc("/v1/mobile/supplier/items", s.handleSupplierItems)
	mux.HandleFunc("/v1/mobile/supplier/dispatch", s.handleCreateDispatch)
	mux.HandleFunc("/v1/mobile/werka/pending", s.handleWerkaPending)
	mux.HandleFunc("/v1/mobile/werka/confirm", s.handleWerkaConfirm)
	mux.HandleFunc("/v1/mobile/admin/settings", s.handleAdminSettings)
	mux.HandleFunc("/v1/mobile/admin/suppliers", s.handleAdminSuppliers)
	mux.HandleFunc("/v1/mobile/admin/suppliers/summary", s.handleAdminSupplierSummary)
	mux.HandleFunc("/v1/mobile/admin/suppliers/detail", s.handleAdminSupplierDetail)
	mux.HandleFunc("/v1/mobile/admin/suppliers/inactive", s.handleAdminInactiveSuppliers)
	mux.HandleFunc("/v1/mobile/admin/suppliers/status", s.handleAdminSupplierStatus)
	mux.HandleFunc("/v1/mobile/admin/suppliers/items", s.handleAdminSupplierItems)
	mux.HandleFunc("/v1/mobile/admin/suppliers/items/assigned", s.handleAdminSupplierAssignedItems)
	mux.HandleFunc("/v1/mobile/admin/suppliers/items/add", s.handleAdminSupplierItemAdd)
	mux.HandleFunc("/v1/mobile/admin/suppliers/items/remove", s.handleAdminSupplierItemRemove)
	mux.HandleFunc("/v1/mobile/admin/suppliers/code/regenerate", s.handleAdminSupplierCodeRegenerate)
	mux.HandleFunc("/v1/mobile/admin/suppliers/remove", s.handleAdminSupplierRemove)
	mux.HandleFunc("/v1/mobile/admin/suppliers/restore", s.handleAdminSupplierRestore)
	mux.HandleFunc("/v1/mobile/admin/items", s.handleAdminItems)
	mux.HandleFunc("/v1/mobile/admin/activity", s.handleAdminActivity)
	mux.HandleFunc("/v1/mobile/admin/werka/code/regenerate", s.handleAdminWerkaCodeRegenerate)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	principal, err := s.auth.Login(r.Context(), strings.TrimSpace(req.Phone), strings.TrimSpace(req.Code))
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials), errors.Is(err, ErrInvalidRole):
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		default:
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		return
	}
	if current, err := s.auth.Profile(r.Context(), principal); err == nil {
		principal = current
	}

	token, err := s.sessions.Create(principal)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "session create failed"})
		return
	}

	writeJSON(w, http.StatusOK, LoginResponse{
		Token:   token,
		Profile: principal,
	})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	token, principal, ok := s.authorizeWithToken(w, r)
	if !ok {
		return
	}
	if current, err := s.auth.Profile(r.Context(), principal); err == nil {
		principal = current
		s.sessions.Update(token, principal)
	}
	writeJSON(w, http.StatusOK, principal)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	token, err := bearerToken(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	s.sessions.Delete(token)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	token, principal, ok := s.authorizeWithToken(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		current, err := s.auth.Profile(r.Context(), principal)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "profile fetch failed"})
			return
		}
		s.sessions.Update(token, current)
		writeJSON(w, http.StatusOK, current)
	case http.MethodPut:
		var req ProfileUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		current, err := s.auth.UpdateNickname(principal, req.Nickname)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "nickname update failed"})
			return
		}
		s.sessions.Update(token, current)
		writeJSON(w, http.StatusOK, current)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleProfileAvatar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	token, principal, ok := s.authorizeWithToken(w, r)
	if !ok {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	if err := r.ParseMultipartForm(6 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid multipart"})
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "avatar is required"})
		return
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "avatar read failed"})
		return
	}

	current, err := s.auth.UploadAvatar(
		r.Context(),
		principal,
		header.Filename,
		header.Header.Get("Content-Type"),
		content,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "avatar upload failed"})
		return
	}

	s.sessions.Update(token, current)
	writeJSON(w, http.StatusOK, current)
}

func (s *Server) handleSupplierHistory(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleSupplier); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	items, err := s.auth.SupplierHistory(r.Context(), principal, 20)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "supplier history failed"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleSupplierItems(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleSupplier); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	items, err := s.auth.SupplierItems(r.Context(), principal, query, 20)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "supplier items failed"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateDispatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleSupplier); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	var req CreateDispatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	record, err := s.auth.CreateDispatch(r.Context(), principal, req.ItemCode, req.Qty)
	if err != nil {
		log.Printf("supplier dispatch create failed for %s/%s: %v", principal.Ref, req.ItemCode, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "dispatch create failed"})
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *Server) handleWerkaPending(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleWerka); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	items, err := s.auth.WerkaPending(r.Context(), 20)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "pending fetch failed"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleWerkaConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleWerka); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	var req ConfirmReceiptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	record, err := s.auth.ConfirmReceipt(r.Context(), req.ReceiptID, req.AcceptedQty)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "receipt confirm failed"})
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *Server) handleAdminSettings(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.auth.AdminSettings())
	case http.MethodPut:
		var req AdminSettings
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if err := s.auth.UpdateAdminSettings(req); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "settings update failed"})
			return
		}
		writeJSON(w, http.StatusOK, s.auth.AdminSettings())
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleAdminSuppliers(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	items, err := s.auth.AdminSuppliers(r.Context(), 100)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "suppliers fetch failed"})
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, items)
		return
	}
	if r.Method == http.MethodPost {
		var req AdminCreateSupplierRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		item, err := s.auth.AdminCreateSupplier(r.Context(), req.Name, req.Phone)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "supplier create failed"})
			return
		}
		writeJSON(w, http.StatusOK, item)
		return
	}
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}

func (s *Server) handleAdminSupplierSummary(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	summary, err := s.auth.AdminSupplierSummary(r.Context(), 300)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "supplier summary failed"})
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleAdminSupplierDetail(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	if ref == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ref is required"})
		return
	}
	detail, err := s.auth.AdminSupplierDetail(r.Context(), ref)
	if err != nil {
		if errors.Is(err, ErrAdminSupplierNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "supplier not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "supplier detail failed"})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleAdminInactiveSuppliers(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	items, err := s.auth.AdminInactiveSuppliers(r.Context(), 300)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "inactive suppliers failed"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleAdminSupplierStatus(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	if ref == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ref is required"})
		return
	}
	var req AdminSupplierStatusUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	detail, err := s.auth.AdminSetSupplierBlocked(r.Context(), ref, req.Blocked)
	if err != nil {
		if errors.Is(err, ErrAdminSupplierNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "supplier not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "supplier status failed"})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleAdminSupplierItems(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	if ref == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ref is required"})
		return
	}
	var req AdminSupplierItemsUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	detail, err := s.auth.AdminUpdateSupplierItems(r.Context(), ref, req.ItemCodes)
	if err != nil {
		if errors.Is(err, ErrAdminSupplierNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "supplier not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "supplier items update failed"})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleAdminSupplierAssignedItems(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	if ref == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ref is required"})
		return
	}
	items, err := s.auth.AdminAssignedSupplierItems(r.Context(), ref, 200)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "assigned items fetch failed"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleAdminSupplierItemAdd(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	if ref == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ref is required"})
		return
	}
	var req AdminSupplierItemMutationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	detail, err := s.auth.AdminAssignSupplierItem(r.Context(), ref, req.ItemCode)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "supplier item add failed"})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleAdminSupplierItemRemove(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	itemCode := strings.TrimSpace(r.URL.Query().Get("item_code"))
	if ref == "" || itemCode == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ref and item_code are required"})
		return
	}
	detail, err := s.auth.AdminUnassignSupplierItem(r.Context(), ref, itemCode)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "supplier item remove failed"})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleAdminSupplierCodeRegenerate(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	if ref == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ref is required"})
		return
	}

	detail, err := s.auth.AdminRegenerateSupplierCode(r.Context(), ref)
	if err != nil {
		if errors.Is(err, ErrAdminSupplierNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "supplier not found"})
			return
		}
		if errors.Is(err, ErrCodeRegenCooldown) {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "code regenerate cooldown"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "supplier code regenerate failed"})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleAdminSupplierRemove(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	if ref == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ref is required"})
		return
	}

	if err := s.auth.AdminRemoveSupplier(r.Context(), ref); err != nil {
		if errors.Is(err, ErrAdminSupplierNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "supplier not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "supplier remove failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleAdminSupplierRestore(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	if ref == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ref is required"})
		return
	}

	detail, err := s.auth.AdminRestoreSupplier(r.Context(), ref)
	if err != nil {
		if errors.Is(err, ErrAdminSupplierNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "supplier not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "supplier restore failed"})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleAdminItems(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		items, err := s.auth.AdminSearchItems(r.Context(), query, 30)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "admin items failed"})
			return
		}
		writeJSON(w, http.StatusOK, items)
	case http.MethodPost:
		var req AdminCreateItemRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		item, err := s.auth.AdminCreateItem(r.Context(), req.Code, req.Name, req.UOM)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "admin item create failed"})
			return
		}
		writeJSON(w, http.StatusOK, item)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleAdminActivity(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	items, err := s.auth.AdminActivity(r.Context(), 30)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "admin activity failed"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleAdminWerkaCodeRegenerate(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	settings, err := s.auth.AdminRegenerateWerkaCode()
	if err != nil {
		if errors.Is(err, ErrCodeRegenCooldown) {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "code regenerate cooldown"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "werka code regenerate failed"})
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) authorize(w http.ResponseWriter, r *http.Request) (Principal, bool) {
	_, principal, ok := s.authorizeWithToken(w, r)
	return principal, ok
}

func (s *Server) authorizeWithToken(w http.ResponseWriter, r *http.Request) (string, Principal, bool) {
	token, err := bearerToken(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return "", Principal{}, false
	}

	principal, ok := s.sessions.Get(token)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return "", Principal{}, false
	}
	return token, principal, true
}

func bearerToken(r *http.Request) (string, error) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(header, "Bearer ") {
		return "", ErrUnauthorized
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if token == "" {
		return "", ErrUnauthorized
	}
	return token, nil
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
