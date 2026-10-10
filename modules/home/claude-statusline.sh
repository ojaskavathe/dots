# Claude Code status line: model · dir[ branch][ worktree] · ctx % (used/max).
# Shared by claude.nix (which puts jq, git, coreutils and sed on PATH) and
# windows/setup.ps1 (Git Bash; jq from winget). Reads the status JSON on stdin.

input=$(cat)

model=$(printf '%s' "$input" | jq -r '.model.display_name')
raw=$(printf '%s' "$input" | jq -r '.workspace.current_dir')
pct=$(printf '%s' "$input" | jq -r '.context_window.used_percentage // 0 | floor')
used=$(printf '%s' "$input" | jq -r '.context_window.total_input_tokens // 0')
max=$(printf '%s' "$input" | jq -r '.context_window.context_window_size // 0')

# Windows passes C:\Users\... ; turn it into /c/Users/... so it matches $HOME
command -v cygpath >/dev/null 2>&1 && raw=$(cygpath -u "$raw")

home_sub() { sed "s|^$HOME|~|"; }

# dir segment: inside a git repo, show the repo folder name (not the deep
# cwd); append the branch when off the default branch, and the worktree
# when this is a linked worktree. Outside git, show the full path. Icons are
# written as UTF-8 byte escapes (branch U+E0A0, worktree U+F487) so no literal
# glyphs live in this source; \u escapes would need a UTF-8 locale, which Git
# Bash on Windows does not set.
seg=""
if toplevel=$(git -C "$raw" rev-parse --show-toplevel 2>/dev/null) && [ -n "$toplevel" ]; then
  gitdir=$(git -C "$raw" rev-parse --absolute-git-dir 2>/dev/null)

  # collapse a worktree checkout back to its parent repo dir
  case "$toplevel" in
    */.worktrees/*) repo_root=$(printf '%s' "$toplevel" | sed 's|/\.worktrees/.*||') ;;
    *)              repo_root=$toplevel ;;
  esac
  disp=$(basename "$repo_root")

  # branch, only when it isn't the repo's default branch
  branch=$(git -C "$raw" branch --show-current 2>/dev/null)
  defbranch=$(git -C "$raw" symbolic-ref --short refs/remotes/origin/HEAD 2>/dev/null | sed 's|^origin/||')
  if [ -z "$defbranch" ]; then
    for c in main master; do
      if git -C "$raw" show-ref --verify --quiet "refs/heads/$c"; then defbranch=$c; break; fi
    done
  fi
  if [ -n "$branch" ] && [ "$branch" != "$defbranch" ]; then
    seg="$seg $(printf '\356\202\240') $branch"
  fi

  # worktree, only when linked (git-dir lives under .../worktrees/<id>)
  case "$gitdir" in
    */worktrees/*) seg="$seg $(printf '\357\222\207') $(basename "$toplevel")" ;;
  esac
else
  disp=$(printf '%s' "$raw" | home_sub)
fi

numfmt() { LC_ALL=en_US.UTF-8 command numfmt --to=si "$@"; }
usedfmt=$(printf '%s' "$used" | numfmt 2>/dev/null || printf '%s' "$used")
maxfmt=$(printf '%s' "$max" | numfmt 2>/dev/null || printf '%s' "$max")

printf '%s · %s%s · ctx %s%% (%s/%s)' "$model" "$disp" "$seg" "$pct" "$usedfmt" "$maxfmt"
