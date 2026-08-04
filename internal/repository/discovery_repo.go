package repository

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type DiscoveryState struct {
	Cursor      string     `json:"cursor"`
	LastAttempt *time.Time `json:"last_attempt"`
	LastSuccess *time.Time `json:"last_success"`
	LastError   *string    `json:"last_error"`
}

type DiscoveryRepo struct{ pool *pgxpool.Pool }

func NewDiscoveryRepo(pool *pgxpool.Pool) *DiscoveryRepo { return &DiscoveryRepo{pool: pool} }

func (r *DiscoveryRepo) DiscoveryCursor(ctx context.Context) (string, error) {
	var cursor string
	if err := r.pool.QueryRow(ctx, `SELECT discovery_cursor FROM show_discovery_state WHERE id = true`).Scan(&cursor); err != nil {
		return "", fmt.Errorf("get discovery cursor: %w", err)
	}
	return cursor, nil
}
func (r *DiscoveryRepo) RecordPollAttempt(ctx context.Context, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE show_discovery_state SET last_poll_attempt_at = $1 WHERE id = true`, at)
	return err
}
func (r *DiscoveryRepo) RecordPollSuccess(ctx context.Context, cursor string, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE show_discovery_state SET discovery_cursor = $1, last_poll_success_at = $2, last_poll_error = NULL WHERE id = true`, cursor, at)
	return err
}
func (r *DiscoveryRepo) RecordPollFailure(ctx context.Context, at time.Time, message string) error {
	_, err := r.pool.Exec(ctx, `UPDATE show_discovery_state SET last_poll_attempt_at = $1, last_poll_error = $2 WHERE id = true`, at, message)
	return err
}
func (r *DiscoveryRepo) Status(ctx context.Context) (*DiscoveryState, error) {
	var s DiscoveryState
	if err := r.pool.QueryRow(ctx, `SELECT discovery_cursor, last_poll_attempt_at, last_poll_success_at, last_poll_error FROM show_discovery_state WHERE id = true`).Scan(&s.Cursor, &s.LastAttempt, &s.LastSuccess, &s.LastError); err != nil {
		return nil, err
	}
	return &s, nil
}
