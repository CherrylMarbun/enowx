package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/enowx/enowx/cloud/api"
)

// A code lives deviceCodeLife; the user code is typed by hand, so it is
// short, upper case, without letters that pass for others or for digits.
const (
	deviceCodeLife = 10 * time.Minute
	userCodeChars  = "BCDFGHJKLMNPQRSTVWXZ"
)

// Errors of signing in with a code.
var (
	ErrPending = errors.New("store: the code isn't approved yet")
	ErrExpired = errors.New("store: the code expired, was used, or never was")
)

// NormalUserCode is code as typed, upper case, without its dash or spaces.
func NormalUserCode(code string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r - 'a' + 'A'
		case r == '-' || r == ' ':
			return -1
		}
		return r
	}, code)
}

// StartDeviceCode starts signing in device p with a code.
func (s *DB) StartDeviceCode(ctx context.Context, p api.DeviceCodeParams) (api.DeviceCode, error) {
	if p.Kind != api.DeviceApp && p.Kind != api.DeviceHost {
		return api.DeviceCode{}, fmt.Errorf("store: a device is %q or %q, not %q", api.DeviceApp, api.DeviceHost, p.Kind)
	}
	if len(p.PublicKey) != 32 {
		return api.DeviceCode{}, errors.New("store: a device's public key is 32 bytes (X25519)")
	}
	s.ExecContext(ctx, `DELETE FROM device_codes WHERE expires < $1`, stamp(now()))
	var raw [32]byte
	rand.Read(raw[:])
	dc := api.DeviceCode{DeviceCode: base64.RawURLEncoding.EncodeToString(raw[:]), Interval: 5, Expires: now().Add(deviceCodeLife)}
	for range 5 { // a user code already given, rarely: another
		var b [8]byte
		rand.Read(b[:])
		code := make([]byte, 8)
		for i, c := range b {
			code[i] = userCodeChars[int(c)%len(userCodeChars)]
		}
		dc.UserCode = string(code[:4]) + "-" + string(code[4:])
		_, err := s.ExecContext(ctx, `INSERT INTO device_codes (hash, user_code, name, kind, public_key, created, expires) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			hashToken(dc.DeviceCode), NormalUserCode(dc.UserCode), strings.TrimSpace(p.Name), p.Kind, p.PublicKey, stamp(now()), stamp(dc.Expires))
		if err == nil {
			return dc, nil
		}
		if !isUnique(err) {
			return api.DeviceCode{}, fmt.Errorf("store: device code: %w", err)
		}
	}
	return api.DeviceCode{}, errors.New("store: no free user code")
}

// PendingDevice is the device showing userCode, waiting for approval.
func (s *DB) PendingDevice(ctx context.Context, userCode string) (api.PendingDevice, error) {
	var d api.PendingDevice
	var created string
	err := s.QueryRowContext(ctx, `SELECT name, kind, created FROM device_codes WHERE user_code = $1 AND expires > $2 AND account_id IS NULL`,
		NormalUserCode(userCode), stamp(now())).Scan(&d.Name, &d.Kind, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return d, ErrExpired
	}
	d.Created = parseStamp(created)
	return d, err
}

// ApproveDeviceCode signs the device showing userCode in to account accID,
// once it asks for its token.
func (s *DB) ApproveDeviceCode(ctx context.Context, userCode, accID string) error {
	res, err := s.ExecContext(ctx, `UPDATE device_codes SET account_id = $1 WHERE user_code = $2 AND expires > $3 AND account_id IS NULL`,
		accID, NormalUserCode(userCode), stamp(now()))
	if err != nil {
		return fmt.Errorf("store: approve device code: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrExpired
	}
	return nil
}

// ClaimDeviceCode is the device of deviceCode signed in, once approved: its
// device and token. The code is used up.
func (s *DB) ClaimDeviceCode(ctx context.Context, deviceCode string) (api.Device, string, string, error) {
	var name, kind, expires string
	var pub []byte
	var accID sql.NullString
	err := s.QueryRowContext(ctx, `SELECT name, kind, public_key, expires, account_id FROM device_codes WHERE hash = $1`, hashToken(deviceCode)).
		Scan(&name, &kind, &pub, &expires, &accID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return api.Device{}, "", "", ErrExpired
	case err != nil:
		return api.Device{}, "", "", err
	case !accID.Valid && now().Before(parseStamp(expires)):
		return api.Device{}, "", "", ErrPending
	case !accID.Valid:
		return api.Device{}, "", "", ErrExpired
	}
	// Used once: the row goes before the device comes, so two asks at once
	// don't make two devices.
	res, err := s.ExecContext(ctx, `DELETE FROM device_codes WHERE hash = $1`, hashToken(deviceCode))
	if err != nil {
		return api.Device{}, "", "", err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return api.Device{}, "", "", ErrExpired
	}
	d, token, err := s.AddDevice(ctx, accID.String, name, kind, pub)
	return d, token, accID.String, err
}
