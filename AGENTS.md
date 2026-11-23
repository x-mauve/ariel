## Code comments and documentation

- Write all code comments in American English, in plain but technical language.
- Write comments the way an engineer writes them in a production codebase: short, direct and specific. No filler, no chatty tone, no narrating the obvious.
- Explain why, not what. Use comments for intent, constraints, invariants, edge cases, side effects and non-obvious trade-offs.
- Code must be well documented by default. Every exported or public package, type, function, method, constant and variable gets a doc comment that follows the language's convention (for example, in Go the comment starts with the identifier name).
- In doc comments, describe parameters, return values, returned errors and, where relevant, concurrency, ownership and lifetime expectations.
- Keep comments in sync with the code. Update or delete a stale comment in the same change that makes it stale.
- Do not leave commented-out code. Do not leave a TODO without context on what is missing and why.

## Commits

- Make every change as atomic commits: one logical change per commit.
- Each commit must leave the project in a working state (it builds and tests pass).
- Do not mix unrelated changes in one commit. Keep features, bug fixes, refactors, formatting and dependency updates in separate commits.
- Write commit messages in American English, imperative mood, with a concise subject line (72 characters or fewer). Add a body only when the reason for the change is not obvious from the diff.

## Versioning and changelog

- The project follows [Semantic Versioning 2.0.0](https://semver.org). Releases are tagged `vMAJOR.MINOR.PATCH`; while the major version is 0, a minor release may contain breaking changes.
- Record every user-visible change in `CHANGELOG.md` in the same commit that makes it. The file follows [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/):
  - New entries go under `## [Unreleased]` at the top.
  - Group entries under `### Added`, `### Changed`, `### Deprecated`, `### Removed`, `### Fixed` and `### Security`, in that order; omit empty groups.
  - Write entries for users of the server, not for its developers: describe the effect, not the implementation.
  - Keep comparison links at the bottom of the file, e.g. `[0.2.0]: https://github.com/x-mauve/ariel/compare/v0.1.0...v0.2.0`.
- To cut a release, rename `[Unreleased]` to `[X.Y.Z] - YYYY-MM-DD`, add a fresh empty `[Unreleased]` section, update the links, commit, then tag `vX.Y.Z`. The release workflow publishes the matching changelog section as the release notes.
