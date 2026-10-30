package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/enowx/enowx/core/designreview"
	"github.com/enowx/enowx/proto"
)

const reviewUsage = `usage: enowx-app review [-|TEXT]   as an independent design reviewer: send your verdict, in this form:
` + designreview.Format

// review sends this session's verdict as a design's reviewer.
func (r runner) review(args []string) error {
	session := os.Getenv("ENOWX_SESSION")
	if len(args) == 0 || session == "" {
		return errors.New(reviewUsage)
	}
	text, err := stdinOr(strings.Join(args, " "))
	if err != nil {
		return err
	}
	rv, err := r.c.DesignReview(proto.DesignReviewParams{Session: session, Text: text})
	if err != nil {
		return err
	}
	fmt.Fprintf(r.out, "Verdict sent: %d/10 (round %d). Its agent is told; you're done.\n", rv.Score, rv.Round)
	return nil
}
