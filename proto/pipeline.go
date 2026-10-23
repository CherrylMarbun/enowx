package proto

import "time"

// Pipelines run agents one after another on one task, each starting from
// what the one before ended with: plan, then implement, then review, say.
// They share a branch, in a worktree of their own. See docs/pipelines.md.
const (
	MethodPipelineStart  = "pipeline.start"  // PipelineStartParams → Pipeline
	MethodPipelineList   = "pipeline.list"   // → []Pipeline
	MethodPipelineCancel = "pipeline.cancel" // PipelineRef
	MethodPipelineRemove = "pipeline.remove" // PipelineRef: forget one that ended
	// EventPipelineUpdated says a pipeline changed: Pipeline.
	EventPipelineUpdated = "pipeline.updated"
)

// PipelineStage is a step of a pipeline: an agent, and its prompt. The
// prompt's {task} is the pipeline's task, {previous} what the step before
// ended with, and {base} the branch the pipeline's started from.
type PipelineStage struct {
	Name   string `json:"name"`
	Agent  string `json:"agent"`
	Prompt string `json:"prompt"`
}

// PipelineStartParams start a pipeline in a project, on a new branch
// (from Base, else the project's current branch).
type PipelineStartParams struct {
	Project         string          `json:"project"`
	Task            string          `json:"task"`
	Branch          string          `json:"branch"`
	Base            string          `json:"base,omitempty"`
	Stages          []PipelineStage `json:"stages"`
	SkipPermissions bool            `json:"skipPermissions,omitempty"`
}

// PipelineRef names a pipeline.
type PipelineRef struct {
	ID string `json:"id"`
}

// Statuses of a pipeline and of its stages.
const (
	PipelinePending   = "pending"   // not started yet
	PipelineRunning   = "running"   // its agent works
	PipelineWaiting   = "waiting"   // its agent asks the user something, in its session
	PipelineDone      = "done"      // finished
	PipelineFailed    = "failed"    // couldn't start, or its session ended first
	PipelineCancelled = "cancelled" // the user stopped it
)

// Pipeline is a pipeline as it runs.
type Pipeline struct {
	ID      string             `json:"id"`
	Project string             `json:"project"`
	Task    string             `json:"task"`
	Branch  string             `json:"branch"`
	Base    string             `json:"base,omitempty"`
	Created time.Time          `json:"created"`
	Status  string             `json:"status"`
	Error   string             `json:"error,omitempty"`
	Skip    bool               `json:"skipPermissions,omitempty"`
	Stages  []PipelineStageRun `json:"stages"`
}

// PipelineStageRun is a stage as it runs: its session, and what its agent
// ended with, handed to the next.
type PipelineStageRun struct {
	PipelineStage
	Status  string    `json:"status"`
	Session string    `json:"session,omitempty"`
	Started time.Time `json:"started,omitzero"`
	Ended   time.Time `json:"ended,omitzero"`
	Output  string    `json:"output,omitempty"`
}

// DefaultPipeline is plan, implement, review, each by agent.
func DefaultPipeline(agent string) []PipelineStage {
	return []PipelineStage{
		{Name: "Plan", Agent: agent, Prompt: "You are the planning step of a pipeline of coding agents. The task:\n\n{task}\n\n" +
			"Read the code and write a plan to carry out the task: what to change, in which files, and how to check it, as a numbered list. Don't change any files; the next step carries the plan out."},
		{Name: "Implement", Agent: agent, Prompt: "You are the implementation step of a pipeline of coding agents. The task:\n\n{task}\n\n" +
			"The planning step's plan:\n\n{previous}\n\nCarry it out, then run the project's tests and checks and fix what fails. End with what you changed."},
		{Name: "Review", Agent: agent, Prompt: "You are the review step of a pipeline of coding agents. The task:\n\n{task}\n\n" +
			"The implementation step reported:\n\n{previous}\n\nReview the changes on this branch against {base} (git diff {base} and the files not committed yet) for bugs, gaps and missing tests. Fix what is wrong, run the checks, and end with what you found and changed."},
	}
}
