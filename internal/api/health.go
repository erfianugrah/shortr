package api

import (
	"net/http"
	"os"

	"github.com/erfianugrah/shortr/internal/identity"
)

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleAPIHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"region":     os.Getenv("FLY_REGION"),
		"app":        os.Getenv("FLY_APP_NAME"),
		"machine_id": os.Getenv("FLY_MACHINE_ID"),
	})
	_ = r
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	p, _ := identity.FromContext(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"subject": p.Subject,
		"method":  p.Method,
	})
}
