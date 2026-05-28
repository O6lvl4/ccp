# ccp — Claude Code Profile

Switch Claude Code configurations like AWS profiles — without duplicating shared settings.

[日本語](./README.ja.md)

## The Problem

Claude Code reads its config from `~/.claude/` by default, but supports `CLAUDE_CONFIG_DIR` to change the location. Switching this directory gives you separate profiles, but **everything** becomes separate: `CLAUDE.md`, `settings.json`, `keybindings.json`, project memory — all of it.

That means your keybindings, shared instructions, and project memory would need to be maintained in every profile independently.

## The Solution

**ccp** uses a symlink-overlay approach. When you create a profile, every file in `~/.claude/` is symlinked into the profile directory. Everything is shared by default — you only break out the specific files you want different per profile.

```
~/.claude/                          # base (shared)
~/.claude-profiles/work/
  CLAUDE.md          → ~/.claude/CLAUDE.md         # shared (symlink)
  settings.json      (real file)                    # overridden
  keybindings.json   → ~/.claude/keybindings.json   # shared (symlink)
  projects/          → ~/.claude/projects/           # shared (symlink)
```

Edit `~/.claude/keybindings.json` once → every profile picks it up. Override `settings.json` in your work profile → only that profile has different permissions/hooks.

**Your original `~/.claude/` is never modified by ccp.** Without any active profile, Claude Code works exactly as before — no setup needed, no side effects. Profiles are purely additive.

## Install

```bash
go install github.com/O6lvl4/ccp@latest
```

### Shell integration

Add to your `.zshrc` or `.bashrc`:

```bash
eval "$(ccp shell-init)"
```

This wraps `ccp switch` so it automatically exports `CLAUDE_CONFIG_DIR` in your current shell.

## Usage

```bash
# Create a profile (symlinks everything from ~/.claude)
ccp init work

# Switch to it (sets CLAUDE_CONFIG_DIR via shell function)
ccp switch work

# Make a file profile-specific (copies from base)
ccp override work settings.json

# Revert a file back to shared (restores symlink)
ccp share work settings.json

# Pick up new files added to ~/.claude since profile creation
ccp sync work

# See what's shared vs overridden
ccp status work
# profile: work
#
#   CLAUDE.md                      shared
#   settings.json                  overridden
#   keybindings.json               shared
#   projects/                      shared

# Switch back to default (~/.claude)
ccp switch

# List all profiles (* = active)
ccp list

# Delete a profile
ccp delete work
```

### Aliases

| Full | Short |
|------|-------|
| `switch` | `sw` |
| `list` | `ls` |
| `status` | `st` |
| `override` | `ov` |
| `share` | `sh` |
| `delete` | `rm` |

## How it works

| Action | What happens |
|--------|-------------|
| `init` | Creates `~/.claude-profiles/<name>/` with symlinks to every entry in `~/.claude/` |
| `switch` | Writes active profile to `~/.claude-profiles/.active`; shell function exports `CLAUDE_CONFIG_DIR` |
| `override` | Replaces a symlink with a real copy of the file/directory |
| `share` | Deletes the real file and restores the symlink to `~/.claude/` |
| `sync` | Adds symlinks for any new files in `~/.claude/` not yet in the profile |

## Requirements

- Go 1.22+
- macOS or Linux
- Claude Code with `CLAUDE_CONFIG_DIR` support
