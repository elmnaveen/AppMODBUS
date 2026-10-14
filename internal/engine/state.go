package engine

import (
	"fmt"
	"strconv"
)

// Definitions returns every connection and poll definition, in order, for
// saving to a workspace.
func (e *Engine) Definitions() ([]ConnectionConfig, []PollConfig) {
	conns := []ConnectionConfig{}
	for _, c := range e.Connections() {
		conns = append(conns, c.ConnectionConfig)
	}
	polls := []PollConfig{}
	for _, p := range e.Polls() {
		polls = append(polls, p.Config)
	}
	return conns, polls
}

// Reset stops and removes all polls and connections.
func (e *Engine) Reset() {
	for _, p := range e.Polls() {
		e.stopPoll(e.poll(p.Config.ID))
	}
	e.mu.Lock()
	conns := e.conns
	e.conns = map[string]*connection{}
	e.polls = map[string]*poll{}
	e.order = nil
	e.nextID = 0
	e.mu.Unlock()
	for _, c := range conns {
		_ = c.client.Close()
	}
	e.emitConnections()
}

// Load replaces the engine contents with saved definitions, keeping their
// IDs. Polls are left stopped. Invalid entries are skipped and reported.
func (e *Engine) Load(conns []ConnectionConfig, polls []PollConfig) (warnings []string) {
	e.loading.Store(true) // loading is not a user change
	defer e.loading.Store(false)

	e.Reset()
	maxID := 0
	track := func(id string) {
		if len(id) > 1 {
			if n, err := strconv.Atoi(id[1:]); err == nil && n > maxID {
				maxID = n
			}
		}
	}
	for _, c := range conns {
		track(c.ID)
	}
	for _, p := range polls {
		track(p.ID)
	}
	e.mu.Lock()
	e.nextID = maxID
	e.mu.Unlock()

	for _, c := range conns {
		if c.ID == "" {
			c.ID = e.lockedID("c")
		}
		if _, err := e.SaveConnection(c); err != nil {
			warnings = append(warnings, fmt.Sprintf("connection %q: %v", c.Name, err))
		}
	}
	for _, cfg := range polls {
		if err := normalizePoll(&cfg); err != nil {
			warnings = append(warnings, fmt.Sprintf("poll window %q: %v", cfg.Name, err))
			continue
		}
		if cfg.ID == "" {
			cfg.ID = e.lockedID("p")
		}
		p := &poll{cfg: cfg, st: PollState{ID: cfg.ID}, stats: newStatsAcc()}
		p.rebuildCells()
		e.mu.Lock()
		if _, dup := e.polls[cfg.ID]; dup {
			e.mu.Unlock()
			warnings = append(warnings, fmt.Sprintf("poll window %q: duplicate id %s", cfg.Name, cfg.ID))
			continue
		}
		e.polls[cfg.ID] = p
		e.order = append(e.order, cfg.ID)
		e.mu.Unlock()
	}
	return warnings
}

func (e *Engine) lockedID(prefix string) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.newID(prefix)
}
