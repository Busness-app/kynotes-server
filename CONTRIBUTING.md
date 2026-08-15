# Contributing to KyNotes

KyNotes is a self-hosted, zero-knowledge note service. Contributions must
preserve the client-side encryption, explicit user actions, and self-hosting
contracts in [DESIGN.md](DESIGN.md).

## Before contributing

Read [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md), [DESIGN.md](DESIGN.md), and
[SECURITY.md](SECURITY.md). Open an issue before proposing a feature larger
than a bug fix or documentation change.

Keep changes small, reuse existing code, and fix shared root causes. New
non-trivial logic needs a runnable test or self-check.

## Security requirements

- The server must never require plaintext notes, attachments, or private keys.
- Destructive actions require an explicit user action.
- Conflicts must fail closed and preserve the rejected encrypted version.
- Do not add telemetry, unconfigured third-party services, or silent security
  fallbacks.
- Document any security trade-off in [DESIGN.md](DESIGN.md) and
  [SECURITY.md](SECURITY.md) before merging.

## Verification

Run the checks that apply to the files changed. At minimum, validate Markdown
links and CSS asset paths, and run the project test suite once implementation
exists. Record the commands and results in the pull request.

## AI attribution

Disclose material AI assistance in the pull request, including the tool, what
it changed, and what you personally reviewed and verified. Add a commit
trailer naming the tool when it materially contributed.

## Pull requests

Use a focused branch and a Conventional Commit subject (`fix:`, `feat:`,
`docs:`, `refactor:`, `test:`, or `chore:`). Include the reason for the change,
verification performed, security impact, and any known limitations.

## Licence

KyNotes is licensed under the GNU Affero General Public License v3.0.
