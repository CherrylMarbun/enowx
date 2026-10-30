# Project search (RAG)

Agents find a project's code, docs, attached notes and memories by meaning, without reading it all. enowX indexes a project with [enowx-rag](https://github.com/enowdev/enowx-rag) (`core/rag`), on the host that has the project, through its daemon. A session on a server works the same way.

## Consent first

Nothing is indexed until two things happen:

1. Settings → **Project search** names an embedder.
2. A project is turned on in the side panel's **Index** tool.

Turning a project on shows what would be sent before anything is (`rag.plan`):

- the embedder and its host;
- how many files and roughly how many tokens;
- what is never sent: secrets (`.env`, keys, files that look like credentials) and what `.gitignore` leaves out;
- a field for more paths to leave out.

**Agree and index** records consent for that embedder. If the embedder changes later, the project waits until the user agrees to the new one. Turning the project off deletes its index and keeps its memories.

## Settings → Project search

The host's settings (`settings/rag.json`) sync with the user's other data ([user-data-sync.md](user-data-sync.md)).

**Embedder**, one of:
- enowx-rag's own `~/.enowx-rag/config.yaml` on the host;
- Voyage AI;
- any OpenAI-compatible `/v1/embeddings` endpoint (OpenAI, Ollama, OpenRouter);
- Text Embeddings Inference.

Each takes a model, dimensions, an address, and the key as a vault credential's name. The daemon asks the app's vault for the key when it embeds, so the key is never written to settings.

**Index kept**, one of:
- on the host: a SQLite file per embedder in the daemon's folder, with nothing else to run;
- on an enowx-rag server (`--serve`), for a team sharing one index. Its token is a vault credential too.

**Test** embeds a line and reports the vector's size.

## Teams

A team's project can be searched on the team's own enowx-rag server, so members share one index. To set this up, in the consent dialog turn on **Search a team's enowx-rag server instead** and fill in:
- the server's address;
- the project's ID there (the project's own ID when left empty);
- the token, as a credential from the member's own vault.

Credentials never go through enowX's cloud, so each member keeps the token in their own vault. The index is built on the server, by whoever runs it. Members' agents only search it: their hosts embed nothing, Reindex is hidden, and turning the project off leaves the server's index as it is.

Each host builds its own indexes; they are large and can be rebuilt, so they are never synced. Only the settings and memories sync.

## The Index tool

The side panel's **Code** group has an **Index** tool for the selected session's project. It shows:

- the index: how many parts, from how many files, when it was last indexed, and with which embedder;
- **Reindex**, and progress (`rag.progress`) while indexing runs;
- a search box. Results show file:line, the symbol and the matching part; clicking one opens the file at those lines, marked;
- the project's memories, which can be added, changed and deleted;
- **Give prompts the parts that bear on them**: at the start of a prompt, the four best results, up to 6000 characters, are added as context. This needs the start-of-prompt hook (Claude Code, Codex).
- **Offer to keep decisions as memories**: when a session ends, a dialog offers the agent's last answer to edit and save (`rag.offer`).
- **Turn off and delete the index**, which asks first.

## Staying fresh

The daemon indexes a project again:
- 15 seconds after a turn ends;
- after a commit it sees (the project's HEAD moved).

Either way it waits until no session in the project is working. Only changed files are embedded again: content hashes are compared, and unchanged parts are skipped. Memories and the notes attached to the project are indexed with it.

## For agents

The feature `rag` (Settings → Features, group Project) is off by default. When turned on, agents get its guide and these:

| CLI | MCP | |
|---|---|---|
| `rag search QUERY [--k N] [--scope code,docs,notes,memory]` | `rag_search` | file:line, symbol and the part found |
| `rag remember TEXT` | `rag_remember` | keeps a decision or gotcha |
| `rag memories`, `rag forget ID` | | lists and removes memories |
| `rag status`, `rag index` | `rag_status` | the index, and reindexing it |

A session reaches only its own project. Outside a session, the CLI uses the current folder, or `--project DIR`.

## Measured

Measured on this repository with `RAG_EVAL=1 go test ./core/rag -run TestEval`, using voyage-code-3 through enowx-rag's settings.

| | |
|---|---|
| First index | 1,436 files, 10,246 parts, about 2.24M tokens embedded, 2 min 15 s |
| Again, a few files changed | 3 parts embedded, 566 tokens, 2 s |
| Again, nothing changed | nothing embedded, 0.5 s |
| Recall@5, 20 questions about the code | 17/20 |
| Read per question | about 900 tokens of results, against whole files for a `git grep -l` of one word (hundreds of thousands of tokens on common words) |

**Recall.** The first run scored 14/20, but three of its expected files were wrong. For example, credentials are lent in `core/vault/lend.go`, not the daemon. The search had found the right file each time; with the expectations corrected, recall is 17/20.

**Misses.** In the three misses, docs that describe the feature ranked above its code. `--scope code` helps when only code is wanted.
