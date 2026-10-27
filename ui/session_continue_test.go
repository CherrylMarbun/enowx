package ui

import (
	"testing"

	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/proto"
)

func TestCanContinue(t *testing.T) {
	cases := []struct {
		name string
		sess proto.SessionInfo
		want bool
	}{
		{
			name: "running session",
			sess: proto.SessionInfo{Exited: false, Agent: "omp"},
			want: false,
		},
		{
			name: "custom command",
			sess: proto.SessionInfo{Exited: true, Agent: agent.Custom},
			want: false,
		},
		{
			name: "empty agent",
			sess: proto.SessionInfo{Exited: true, Agent: ""},
			want: false,
		},
		{
			name: "shell session",
			sess: proto.SessionInfo{Exited: true, Agent: agent.Shell},
			want: true,
		},
		{
			name: "omp agent with continue",
			sess: proto.SessionInfo{Exited: true, Agent: "omp"},
			want: true,
		},
		{
			name: "claude agent with continue",
			sess: proto.SessionInfo{Exited: true, Agent: "claude"},
			want: true,
		},
		{
			name: "agent without continue flag",
			sess: proto.SessionInfo{Exited: true, Agent: "pi"},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := canContinue(tc.sess); got != tc.want {
				t.Errorf("canContinue(%+v) = %v, want %v", tc.sess, got, tc.want)
			}
		})
	}
}
