package api

import (
	"context"
	"os"
	"sort"

	"pulsenode/backend/internal/docker"
)

// externalIDPrefix marks a project ID as referring to a container discovered
// on the host rather than a row in the projects table — see discoverExternalProjects.
const externalIDPrefix = "ext:"

// externalProject describes an app already running on the VPS (behind
// Traefik, with its own domain) that wasn't deployed through PulseNode —
// e.g. a site set up by hand with plain docker compose. It reuses the
// project list/detail JSON shape so the frontend can render it alongside
// real projects; fields with no equivalent (RepoURL, Branch, ...) are "".
type externalProject struct {
	ID          string `json:"ID"`
	Name        string `json:"Name"`
	RepoURL     string `json:"RepoURL"`
	Branch      string `json:"Branch"`
	Domain      string `json:"Domain"`
	Status      string `json:"Status"`
	BuildMethod string `json:"BuildMethod"`
	BaseDir     string `json:"BaseDir"`
	CreatedAt   string `json:"CreatedAt"`
	Image       string `json:"Image"`
	Ports       string `json:"Ports"`
	ContainerID string `json:"ContainerID"`
	External    bool   `json:"External"`
}

// selfComposeProject identifies the docker-compose project this go-api
// process itself belongs to, by matching its own container ID (the process's
// hostname, which Docker sets to the short container ID by default) against
// the discovered containers. Used to exclude PulseNode's own stack
// (web/caddy/go-api) from "external projects" — it's the tool, not something
// hosted on the VPS. Returns "" if it can't be determined (e.g. dev/non-container run).
func selfComposeProject(all []docker.ContainerLabels) string {
	hostname, err := os.Hostname()
	if err != nil {
		return ""
	}
	for _, c := range all {
		if c.ID == hostname {
			return c.Labels["com.docker.compose.project"]
		}
	}
	return ""
}

// discoverExternalProjects lists running containers exposed via a Traefik
// domain that aren't already tracked as a PulseNode project (no
// pulsenode.project label, and not the container behind any project row) or
// part of PulseNode's own stack.
func (s *Server) discoverExternalProjects(ctx context.Context) []externalProject {
	if s.docker == nil {
		return nil
	}
	containers, err := s.docker.Containers(ctx)
	if err != nil {
		return nil
	}
	labeled, err := s.docker.ContainersWithLabels(ctx)
	if err != nil {
		return nil
	}
	byName := make(map[string]docker.ContainerLabels, len(labeled))
	for _, c := range labeled {
		byName[c.Name] = c
	}
	self := selfComposeProject(labeled)

	managed := map[string]bool{}
	if s.db != nil {
		if projects, err := s.db.ListProjects(); err == nil {
			for _, p := range projects {
				if p.ContainerID != "" {
					managed[p.ContainerID] = true
				}
			}
		}
	}

	out := []externalProject{}
	for _, ctr := range containers {
		lbl, ok := byName[ctr.Name]
		if !ok || lbl.Labels["pulsenode.project"] != "" || managed[ctr.ID] {
			continue
		}
		if self != "" && lbl.Labels["com.docker.compose.project"] == self {
			continue
		}
		hosts := parseTraefikHosts(lbl.Labels)
		if len(hosts) == 0 {
			continue
		}
		out = append(out, externalProject{
			ID:          externalIDPrefix + ctr.ID,
			Name:        ctr.Name,
			Domain:      hosts[0],
			Status:      ctr.State,
			CreatedAt:   ctr.Created,
			Image:       ctr.Image,
			Ports:       ctr.Ports,
			ContainerID: ctr.ID,
			External:    true,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// getExternalProject looks up a single discovered external project by its
// "ext:<containerID>" ID.
func (s *Server) getExternalProject(ctx context.Context, id string) (*externalProject, bool) {
	for _, p := range s.discoverExternalProjects(ctx) {
		if p.ID == id {
			return &p, true
		}
	}
	return nil, false
}
