package agent

// CredsGuide tells an agent how to use the user's credential vault: by
// name, never seeing a value.
const CredsGuide = `Credentials: the user keeps logins (GitHub, cloud providers, databases, AI APIs, SSH keys...) in enowX's vault, in the app on their own computer. You never see their values; use them by name:
    "$ENOWX_BIN" creds                         # what the vault holds: names, platforms, the variables each sets (no values)
    "$ENOWX_BIN" creds use aws-prod -- aws s3 ls   # run a command with a credential's variables; what it prints is masked
    "$ENOWX_BIN" creds env github              # this session's programs (and creds use without a name) get its variables from now on
    "$ENOWX_BIN" creds login cloudflare        # ask the user to log in to a platform, in the app
    "$ENOWX_BIN" creds add NAME --platform P --value -   # keep a new token in the vault (the user confirms); creds test NAME, creds rm NAME
git, docker and ssh in a session already get the vault's logins (the app asks the user the first time on each host). Never print, echo or write a credential's value, and don't ask the user to paste a secret into the chat: have them add it in the app (creds login).`
