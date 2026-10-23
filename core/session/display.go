package session

// SetDisplay records the port of the virtual display s's programs can
// open their windows on, given after it started.
func (m *Manager) SetDisplay(s *Session, port, scale int) {
	s.mu.Lock()
	s.meta.Display, s.meta.DisplayScale = port, scale
	s.mu.Unlock()
	m.notify(s)
}
