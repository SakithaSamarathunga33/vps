package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

func (s *Server) requireDocker(w http.ResponseWriter) bool {
	if s.docker == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Docker unavailable"})
		return false
	}
	return true
}

func (s *Server) dockerContainers(w http.ResponseWriter, r *http.Request) {
	if !s.requireDocker(w) {
		return
	}
	items, err := s.docker.Containers(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) dockerImages(w http.ResponseWriter, r *http.Request) {
	if !s.requireDocker(w) {
		return
	}
	items, err := s.docker.Images(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) dockerNetworks(w http.ResponseWriter, r *http.Request) {
	if !s.requireDocker(w) {
		return
	}
	items, err := s.docker.Networks(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) dockerDatabases(w http.ResponseWriter, r *http.Request) {
	if !s.requireDocker(w) {
		return
	}
	items, err := s.docker.DatabaseContainers(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) dockerLogs(w http.ResponseWriter, r *http.Request) {
	if !s.requireDocker(w) {
		return
	}
	tail, _ := strconv.Atoi(r.URL.Query().Get("tail"))
	if tail <= 0 {
		tail = 100
	}
	logs, err := s.docker.Logs(r.Context(), chi.URLParam(r, "id"), tail)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"logs": logs})
}

func (s *Server) dockerAction(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.requireDocker(w) {
			return
		}
		if err := s.docker.Action(r.Context(), chi.URLParam(r, "id"), action); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

func (s *Server) dockerExec(w http.ResponseWriter, r *http.Request) {
	if !s.requireDocker(w) {
		return
	}
	var body struct {
		Cmd string `json:"cmd"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Cmd == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cmd required"})
		return
	}
	output, err := s.docker.Exec(r.Context(), chi.URLParam(r, "id"), body.Cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"output": output})
}

func (s *Server) dockerPullImage(w http.ResponseWriter, r *http.Request) {
	if !s.requireDocker(w) {
		return
	}
	var body struct {
		Image string `json:"image"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Image == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "image is required"})
		return
	}
	if err := s.docker.PullImage(r.Context(), body.Image); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "image": body.Image})
}

func (s *Server) dockerPruneImages(w http.ResponseWriter, r *http.Request) {
	if !s.requireDocker(w) {
		return
	}
	reclaimed, err := s.docker.PruneImages(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "reclaimedBytes": reclaimed, "reclaimedMB": fmt.Sprintf("%.1f", float64(reclaimed)/1024/1024)})
}

func (s *Server) clearBuildCache(w http.ResponseWriter, r *http.Request) {
	if !s.requireDocker(w) {
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	send := func(payload string) {
		_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
	send(`{"type":"line","text":"Pruning build cache via Docker API..."}`)
	reclaimed, err := s.docker.PruneBuildCache(r.Context())
	if err != nil {
		send(fmt.Sprintf(`{"type":"error","text":%q}`, err.Error()))
		return
	}
	send(fmt.Sprintf(`{"type":"line","text":"Space reclaimed: %.1f MB"}`, float64(reclaimed)/1024/1024))
	send(`{"type":"done"}`)
}

type heartbeatPoint struct {
	Up bool      `json:"up"`
	At time.Time `json:"at"`
}

type containerHeartbeats struct {
	Name  string           `json:"name"`
	Beats []heartbeatPoint `json:"beats"`
}

// dockerHeartbeats returns each currently-existing container's recorded
// up/down history for the requested window (?since=24h|3d|7d, default 24h).
func (s *Server) dockerHeartbeats(w http.ResponseWriter, r *http.Request) {
	if !s.requireDocker(w) {
		return
	}
	window := 24 * time.Hour
	switch r.URL.Query().Get("since") {
	case "3d":
		window = 3 * 24 * time.Hour
	case "7d":
		window = 7 * 24 * time.Hour
	}

	containers, err := s.docker.Containers(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	beats, err := s.db.ListHeartbeatsSince(time.Now().Add(-window))
	if err != nil {
		writeError(w, err)
		return
	}
	byName := make(map[string][]heartbeatPoint, len(containers))
	for _, b := range beats {
		byName[b.ContainerName] = append(byName[b.ContainerName], heartbeatPoint{Up: b.Up, At: b.CheckedAt})
	}

	out := make([]containerHeartbeats, 0, len(containers))
	for _, c := range containers {
		points := byName[c.Name]
		if points == nil {
			points = []heartbeatPoint{}
		}
		out = append(out, containerHeartbeats{Name: c.Name, Beats: points})
	}
	writeJSON(w, http.StatusOK, out)
}
