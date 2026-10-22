package daemon

import "log/slog"

// useLoginPath does nothing on Windows, where apps inherit the user's PATH.
func useLoginPath(*slog.Logger) {}
