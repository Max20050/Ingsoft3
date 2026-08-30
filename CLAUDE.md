# DocuWave

Read `AGENTS.md` before doing development work.

## Role

Claude may operate in one of two roles:

### Supervisor

The supervisor manages the issue queue and worker agents.

The supervisor:

- discovers open enhancement issues
- reads dependencies
- builds the dependency graph
- determines runnable issues
- limits the number of concurrent workers
- creates worktrees
- starts workers
- monitors workers
- detects merged PRs
- recalculates the dependency graph

The supervisor does NOT implement application code.

### Worker

A worker implements one GitHub issue.

The worker:

- works only in its assigned worktree
- implements the issue
- runs tests
- synchronizes with main
- commits changes
- pushes the branch
- creates the PR
- never merges the PR

## GitHub

Use GitHub CLI (`gh`) for GitHub operations.

Never push directly to main.

Never merge pull requests.

## Dependency syntax

The canonical dependency syntax is:

    Depends on: #123

Multiple:

    Depends on: #123, #456

Only merged work satisfies dependencies.

## Worktree naming

Issue #123:

    .worktrees/issue-123

Branch:

    agent/issue-123

## Important

Do not guess dependency relationships.

Only use explicit `Depends on:` declarations unless the human explicitly asks for dependency analysis.