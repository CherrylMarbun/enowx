package cli

// credsTools use the user's credential vault, by name, never a value.
var credsTools = []mcpTool{
	{name: "creds_list", description: "The credentials the user keeps in enowX's vault: names, platforms, accounts, statuses and the environment variables each sets. Never a value.",
		args: func(map[string]any) []string { return []string{"list"} }},
	{name: "creds_run", description: "Run a shell command with credentials' environment variables (by name; none: those lent to this session). The values never reach you: what the command prints is masked. The app asks the user the first time a credential is used on a host.",
		props:    map[string]mcpProp{"names": {"array", "The credentials' names (creds_list); empty for this session's"}, "command": {"string", "The shell command line to run"}},
		required: []string{"command"},
		args: func(a map[string]any) []string {
			args := append([]string{"use"}, strs(a, "names")...)
			return append(append(args, "--"), shellCommand(str(a, "command"))...)
		}},
	{name: "creds_env", description: "Lend credentials to this session: the programs it starts from now on (and creds_run without names) get their variables.",
		props: map[string]mcpProp{"names": {"array", "The credentials' names"}}, required: []string{"names"},
		args: func(a map[string]any) []string { return append([]string{"env"}, strs(a, "names")...) }},
	{name: "creds_add", description: "Keep a new credential in the vault (a token you were given or made); the user confirms in the app.",
		props: map[string]mcpProp{"name": {"string", "Its name, no spaces"}, "platform": {"string", "Its platform's ID (creds_platforms)"},
			"account": {"string", "The account it logs in as"}, "value": {"string", "Its main value (the token, key or password)"}},
		required: []string{"name", "platform", "value"},
		args: func(a map[string]any) []string {
			args := withFlag([]string{"add", str(a, "name")}, a, "platform", "platform")
			args = withFlag(args, a, "account", "account")
			return withFlag(args, a, "value", "value")
		}},
	{name: "creds_test", description: "Check a credential still works, with its platform: valid, invalid, expired or revoked.",
		props: map[string]mcpProp{"name": {"string", "The credential's name"}}, required: []string{"name"},
		args: func(a map[string]any) []string { return []string{"test", str(a, "name")} }},
	{name: "creds_remove", description: "Remove a credential from the vault; the user confirms in the app.",
		props: map[string]mcpProp{"name": {"string", "The credential's name"}}, required: []string{"name"},
		args: func(a map[string]any) []string { return []string{"rm", str(a, "name")} }},
	{name: "creds_login", description: "Ask the user to log in to a platform in the app (a token to paste, or the platform's own login), to add it to the vault.",
		props: map[string]mcpProp{"platform": {"string", "The platform's ID (creds_platforms)"}}, required: []string{"platform"},
		args: func(a map[string]any) []string { return []string{"login", str(a, "platform")} }},
	{name: "creds_audit", description: "The credentials used on this host lately: which, when, for what, allowed or refused.",
		args: func(map[string]any) []string { return []string{"audit"} }},
	{name: "creds_platforms", description: "The platforms the vault knows (their IDs) and the variables each sets.",
		props: map[string]mcpProp{"query": {"string", "Part of a platform's name"}},
		args:  func(a map[string]any) []string { return []string{"platforms", str(a, "query")} }},
}

// strs is an array argument's strings.
func strs(a map[string]any, k string) []string {
	var out []string
	switch v := a[k].(type) {
	case []any:
		for _, x := range v {
			if s, ok := x.(string); ok && s != "" {
				out = append(out, s)
			}
		}
	case string:
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}
