# DocuWave Agent Instructions

## Repository workflow

DocuWave uses GitHub Flow.

`main` is the only permanent development branch.

Agents work on isolated issue branches:

    agent/issue-{number}

Every issue must have its own branch and worktree.

## Worktrees

Agents must never implement work directly in the main checkout.

Each issue must use:

    .worktrees/issue-{number}

Example:

    .worktrees/issue-42
    agent/issue-42

## Scope

An agent must implement only the assigned issue.

Do not:

- implement unrelated issues
- perform unrelated refactors
- modify another agent's worktree
- merge another branch
- push directly to main

## Dependencies

Issues may declare dependencies in their body:

    Depends on: #5

Multiple dependencies:

    Depends on: #5, #7, #12

An issue is considered blocked until all dependencies have reached main.

A PR being open does NOT satisfy a dependency.

Only a merged PR satisfies a dependency.

## Pull requests

Each issue gets its own pull request.

Pull requests target:

    main

Do not combine unrelated issues into one PR.

## Before creating a PR

The worker must:

1. Fetch origin/main.
2. Rebase onto origin/main.
3. Resolve conflicts when the correct resolution is clear.
4. Run tests.
5. Review the final diff.
6. Commit.
7. Push.
8. Create the PR.

## Existing PRs

If main changes while a PR is open, the worker should update its branch:

    git fetch origin
    git rebase origin/main

Then run tests again.

## Merge authority

Agents never merge pull requests.

The repository owner reviews and merges PRs into main.

## Completion

A worker is finished when:

- implementation is complete
- tests pass
- branch is synchronized with main
- PR exists
- PR is ready for human review

The worker must not claim the issue is completed until its PR is merged.