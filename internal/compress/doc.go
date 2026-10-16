// Package compress shortens the output of Bash tool calls (ADR-0007). Only
// tool_result blocks whose tool_use is a Bash call are touched; the rule is
// chosen by the command of that tool_use. All strategies are pure functions,
// so every turn compresses the whole history to the same bytes and the
// prompt cache prefix stays stable.
package compress
