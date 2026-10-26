package daemon

import (
	"encoding/json"
	"os"
	"os/exec"
	"reflect"
	"testing"

	"github.com/enowx/enowx/core/agent"
)

// A program started through PowerShell, as thenShell starts agents, gets
// its arguments as they were: quotes, $, backslashes, newlines and empty
// ones, in Windows PowerShell 5.1 and in PowerShell 7 where it is
// installed.
func TestPsQuotePreservesArguments(t *testing.T) {
	cases := [][]string{
		{"--append-system-prompt", agent.Guide},
		{"simple", "words"},
		{"don't", "it's 'great'"},
		{`say "hello"`, `nested "quotes" inside`},
		{`'hello' and "world"`, `"don't" do that`},
		{`eval "$("$ENOWX_BIN" display)"`},
		{"$VAR", "$1", "$@", "${VAR}"},
		{"line 1\nline 2\r\nline 3\ttab"},
		{`path\to\"file"`, `C:\Program Files\app`},
		{"", "next"},
	}
	for _, shell := range []string{"powershell.exe", "pwsh.exe"} {
		if _, err := exec.LookPath(shell); err != nil {
			t.Logf("%s isn't installed", shell)
			continue
		}
		for _, tc := range cases {
			run := psLegacy + "& " + psQuote(append([]string{os.Args[0]}, tc...))
			cmd := exec.Command(shell, "-NoLogo", "-NoProfile", "-Command", run)
			cmd.Env = append(os.Environ(), psArgsHelper+"=1")
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("%s: %v: %s", shell, err, out)
			}
			var got []string
			if err := json.Unmarshal(out, &got); err != nil {
				t.Fatalf("%s: the program printed %q: %v", shell, out, err)
			}
			if !reflect.DeepEqual(got, tc) {
				t.Errorf("%s:\n got: %#v\nwant: %#v", shell, got, tc)
			}
		}
	}
}
