package vault

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Logging in in the browser (OAuth's device flow): the app asks the
// platform for a code, the user types it on the platform's page, signed
// in there, and the app gets a token for the vault. It takes an OAuth app
// of the user's own (its client ID): enowX has none of its own to log in
// through.

// Device is how a platform logs in by the device flow.
type Device struct {
	CodeURL  string // where codes are asked for
	TokenURL string // where the token is asked for, once the user agrees
	Scope    string
	// Field is the credential's field the token goes in.
	Field string
	// Help says where the user makes the OAuth app whose client ID it
	// takes.
	Help string
}

// DeviceCode is what the user types, and where.
type DeviceCode struct {
	UserCode string
	URL      string // the page to type it on
	code     string
	interval time.Duration
	expires  time.Time
	d        Device
	clientID string
}

// ErrDeviceDenied: the user declined on the platform's page.
var ErrDeviceDenied = errors.New("vault: the login was declined on the platform's page")

// StartDevice asks d's platform for a code for OAuth app clientID.
func StartDevice(ctx context.Context, d Device, clientID string) (*DeviceCode, error) {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return nil, errors.New("vault: the browser's login needs your OAuth app's client ID: " + d.Help)
	}
	var out struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		ExpiresIn       int    `json:"expires_in"`
		Interval        int    `json:"interval"`
		Error           string `json:"error"`
		Description     string `json:"error_description"`
	}
	if err := devicePost(ctx, d.CodeURL, url.Values{"client_id": {clientID}, "scope": {d.Scope}}, &out); err != nil {
		return nil, err
	}
	if out.Error != "" || out.DeviceCode == "" {
		return nil, fmt.Errorf("vault: the platform gave no code: %s", orText(out.Description, out.Error))
	}
	dc := &DeviceCode{UserCode: out.UserCode, URL: out.VerificationURI, code: out.DeviceCode, d: d, clientID: clientID,
		interval: time.Duration(max(out.Interval, 1)) * time.Second, expires: time.Now().Add(time.Duration(max(out.ExpiresIn, 60)) * time.Second)}
	return dc, nil
}

// Wait waits for the user to type the code and agree, and returns the
// token; ctx ends it sooner.
func (dc *DeviceCode) Wait(ctx context.Context) (string, error) {
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(dc.interval):
		}
		if time.Now().After(dc.expires) {
			return "", errors.New("vault: the code expired before it was typed: start again")
		}
		var out struct {
			AccessToken string `json:"access_token"`
			Error       string `json:"error"`
			Description string `json:"error_description"`
			Interval    int    `json:"interval"`
		}
		err := devicePost(ctx, dc.d.TokenURL, url.Values{"client_id": {dc.clientID}, "device_code": {dc.code},
			"grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}}, &out)
		if err != nil {
			return "", err
		}
		switch out.Error {
		case "":
			if out.AccessToken == "" {
				return "", errors.New("vault: the platform answered without a token")
			}
			return out.AccessToken, nil
		case "authorization_pending":
		case "slow_down":
			dc.interval += 5 * time.Second
			if out.Interval > 0 {
				dc.interval = time.Duration(out.Interval) * time.Second
			}
		case "access_denied":
			return "", ErrDeviceDenied
		case "expired_token":
			return "", errors.New("vault: the code expired before it was typed: start again")
		default:
			return "", fmt.Errorf("vault: the platform refused the login: %s", orText(out.Description, out.Error))
		}
	}
}

// devicePost posts form to u and reads its JSON answer, errors included
// (OAuth's come with 400).
func devicePost(ctx context.Context, u string, form url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("vault: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 500 {
		return fmt.Errorf("vault: the platform answered %s", res.Status)
	}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return fmt.Errorf("vault: the platform's answer (%s): %w", res.Status, err)
	}
	return nil
}

func orText(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
