package daemon

import "errors"

// exeSuffix is a program's file extension.
const exeSuffix = ".exe"

// startSSHAgent: Windows' ssh talks to an agent over a named pipe, which
// the vault doesn't serve yet; keys stay usable through enowx creds.
func (s *Server) startSSHAgent(string) (string, func(), error) {
	return "", nil, errors.New("the vault's SSH agent isn't on Windows yet")
}
