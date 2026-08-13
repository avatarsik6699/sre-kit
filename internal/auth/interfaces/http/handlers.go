// Package http exposes POST /api/auth/login and the session-required middleware every other
// endpoint is mounted behind (docs/SPEC.md §6): "every REST/WS endpoint except /api/auth/login and
// a health-check endpoint requires a valid session."
package http

import (
	"encoding/json"
	"net/http"

	"sre-kit/internal/auth/application"
	"sre-kit/internal/platform/apierror"
)

// sessionCookieName is the HttpOnly cookie the session token travels in.
const sessionCookieName = "session"

// Handlers exposes the auth HTTP surface bound to a *application.Service.
type Handlers struct {
	service *application.Service
}

// NewHandlers wires Handlers to svc.
func NewHandlers(svc *application.Service) *Handlers {
	return &Handlers{service: svc}
}

// Register mounts POST /api/auth/login on mux.
func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/login", h.login)
}

type loginRequest struct {
	Password string `json:"password"`
}

// login godoc
// @Summary      Log in
// @Description  Exchange the admin password for a session cookie
// @Tags         auth
// @Accept       json
// @Param        credentials  body  http.loginRequest  true  "admin password"
// @Success      204
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Router       /api/auth/login [post]
func (h *Handlers) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.Write(w, apierror.Invalid("malformed request body"))
		return
	}

	session, err := h.service.Login(r.Context(), req.Password)
	if err != nil {
		apierror.Write(w, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    session.Token,
		Path:     "/",
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}
