# magos

An AI coding assistant that runs in the terminal. Sends messages to Claude via the Anthropic API, executes tool calls (`read_files`, `modify_files`) against your local codebase, and applies changes through git worktrees. Activity is logged to `.magos/logs.db`.

Requires Go 1.25+, a C compiler (for go-sqlite3), and `MAGOS_ANTHROPIC_API_KEY` set in your environment. Run from inside a git repository.
