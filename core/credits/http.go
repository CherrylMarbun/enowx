package credits

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/enowx/enowx/version"
)

var httpClient = &http.Client{Timeout: 20 * time.Second}

// errSignedOut is a token the vendor refused: the agent renews it the
// next time it runs.
var errSignedOut = errors.New("sign-in expired: run the agent once to renew it, or sign in again")

// getJSON GETs url with headers and decodes the answer into out. Errors
// never carry the headers, which hold the token.
func getJSON(ctx context.Context, url string, headers map[string]string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "enowX/"+version.String)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not reach %s", req.URL.Host)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("could not read %s's answer", req.URL.Host)
	}
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return errSignedOut
	case resp.StatusCode == http.StatusTooManyRequests:
		return fmt.Errorf("%s is limiting requests; trying again later", req.URL.Host)
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("%s answered %s", req.URL.Host, resp.Status)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("%s's answer changed shape", req.URL.Host)
	}
	return nil
}

// windowLabel names a limit by its length.
func windowLabel(d time.Duration) string {
	switch {
	case d <= 0:
		return "Limit"
	case d == 5*time.Hour:
		return "5 hours"
	case d == 7*24*time.Hour:
		return "Week"
	case d%(24*time.Hour) == 0:
		return fmt.Sprintf("%d days", int(d/(24*time.Hour)))
	case d%time.Hour == 0:
		return fmt.Sprintf("%d hours", int(d/time.Hour))
	}
	return d.String()
}
