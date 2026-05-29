package api

import "net/http"

// serveIndex returns the SPA shell — the dashboard's index.html. Used for
// any client-side route the server doesn't otherwise handle (the catch-all
// {slug} pattern handles redirects, but explicit dashboard routes like /
// and /login fall through to here).
func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	if s.staticFS == nil {
		http.NotFound(w, r)
		return
	}
	f, err := s.staticFS.Open("/index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeContent(w, r, "index.html", stat.ModTime(), f)
}
