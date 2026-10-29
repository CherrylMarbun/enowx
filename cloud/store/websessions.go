package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"time"

	"github.com/enowx/enowx/cloud/api"
)

// webSessionLife is how long a browser stays signed in to the cloud's
// pages: long enough to approve a device, not for good.
const webSessionLife = 12 * time.Hour

// StartWebSession signs account accID in to the cloud's pages; the token
// goes in the browser's cookie, its hash in the database.
func (s *DB) StartWebSession(ctx context.Context, accID string) (string, error) {
	s.ExecContext(ctx, `DELETE FROM web_sessions WHERE expires < $1`, stamp(now()))
	var raw [32]byte
	rand.Read(raw[:])
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	_, err := s.ExecContext(ctx, `INSERT INTO web_sessions (hash, account_id, created, expires) VALUES ($1, $2, $3, $4)`,
		hashToken(token), accID, stamp(now()), stamp(now().Add(webSessionLife)))
	return token, err
}

// WebSessionAccount is the account a browser's token is signed in as,
// ErrNotFound when it isn't, or no longer.
func (s *DB) WebSessionAccount(ctx context.Context, token string) (api.Account, error) {
	var accID string
	err := s.QueryRowContext(ctx, `SELECT account_id FROM web_sessions WHERE hash = $1 AND expires > $2`, hashToken(token), stamp(now())).Scan(&accID)
	if errors.Is(err, sql.ErrNoRows) {
		return api.Account{}, ErrNotFound
	}
	if err != nil {
		return api.Account{}, err
	}
	return s.Account(ctx, accID)
}

// EndWebSession signs a browser out.
func (s *DB) EndWebSession(ctx context.Context, token string) error {
	_, err := s.ExecContext(ctx, `DELETE FROM web_sessions WHERE hash = $1`, hashToken(token))
	return err
}
