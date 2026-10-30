package state

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func (s *Store) CreateAgentToken(t AgentToken, tokenHash string) error {
	_, err := s.db.Exec(`INSERT INTO agent_tokens (id, name, token_hash, created_at) VALUES (?, ?, ?, ?)`,
		t.ID, t.Name, tokenHash, t.CreatedAt)
	return err
}

func (s *Store) ListAgentTokens() []AgentToken {
	rows, err := s.db.Query(`SELECT id, name, COALESCE(agent_id, ''), created_at, last_used_at
		FROM agent_tokens ORDER BY created_at DESC`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []AgentToken{}
	for rows.Next() {
		var t AgentToken
		if err := rows.Scan(&t.ID, &t.Name, &t.AgentID, &t.CreatedAt, &t.LastUsedAt); err == nil {
			out = append(out, t)
		}
	}
	return out
}

// AgentTokenByHash returns the token with this SHA-256, or false.
func (s *Store) AgentTokenByHash(tokenHash string) (AgentToken, bool) {
	var t AgentToken
	err := s.db.QueryRow(`SELECT id, name, COALESCE(agent_id, ''), created_at, last_used_at
		FROM agent_tokens WHERE token_hash = ?`, tokenHash).
		Scan(&t.ID, &t.Name, &t.AgentID, &t.CreatedAt, &t.LastUsedAt)
	return t, err == nil
}

// TouchAgentToken records use and binds the token to agentID if still unbound.
func (s *Store) TouchAgentToken(id, agentID string) error {
	_, err := s.db.Exec(`UPDATE agent_tokens SET last_used_at = ?,
		agent_id = CASE WHEN COALESCE(agent_id, '') = '' THEN ? ELSE agent_id END WHERE id = ?`,
		time.Now().UTC(), agentID, id)
	return err
}

// DeleteAgentToken removes the token and returns the agent it was bound to.
func (s *Store) DeleteAgentToken(id string) (string, error) {
	var agentID string
	err := s.db.QueryRow(`SELECT COALESCE(agent_id, '') FROM agent_tokens WHERE id = ?`, id).Scan(&agentID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("agent token %q not found", id)
	}
	if err != nil {
		return "", err
	}
	_, err = s.db.Exec(`DELETE FROM agent_tokens WHERE id = ?`, id)
	return agentID, err
}
