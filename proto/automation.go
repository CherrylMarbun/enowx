package proto

import "time"

// Automations start an agent on a prompt on a schedule, in a session of
// their own, whether or not an app is connected. See docs/automations.md.
const (
	MethodAutomationList   = "automation.list"   // → []Automation
	MethodAutomationSave   = "automation.save"   // Automation → Automation (new when ID is "")
	MethodAutomationRemove = "automation.remove" // AutomationRef
	MethodAutomationRun    = "automation.run"    // AutomationRef → SessionInfo, now
)

// Automation is a prompt an agent gets on a schedule.
type Automation struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Project string `json:"project"`
	Agent   string `json:"agent"`
	Prompt  string `json:"prompt"`
	// Schedule says when, as "hourly", "daily 09:00", "weekdays 18:30" or
	// "weekly mon 08:00", in the host's time.
	Schedule string `json:"schedule"`
	// Branch runs each time on a new branch, in a worktree of its own;
	// else in the project's folder.
	Branch          bool `json:"branch,omitempty"`
	SkipPermissions bool `json:"skipPermissions,omitempty"`
	Enabled         bool `json:"enabled"`
	// What the daemon keeps: the last run, its session or why it failed,
	// and the next.
	LastRun     time.Time `json:"lastRun,omitzero"`
	LastSession string    `json:"lastSession,omitempty"`
	LastError   string    `json:"lastError,omitempty"`
	NextRun     time.Time `json:"nextRun,omitzero"`
}

// AutomationRef names an automation.
type AutomationRef struct {
	ID string `json:"id"`
}
