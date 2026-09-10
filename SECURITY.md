# Security Policy

Argos is maintained by a single maintainer. This policy is intentionally simple to
match that reality - no formal security team, no bug bounty program.

## Reporting a vulnerability

**Do not open a public GitHub issue for a suspected vulnerability.** Instead, use
[GitHub's private vulnerability reporting](https://github.com/jhonsferg/argos/security/advisories/new)
for this repository (Security tab -> Report a vulnerability). This opens a private
advisory only the maintainer can see until it's resolved.

Include, if known:

- The affected module(s) and version(s) (each `integrations/*` module versions
  independently - see [Supported versions](#supported-versions)).
- A minimal reproduction or proof of concept.
- The impact you believe it has (e.g. data exposure, panic/DoS, privilege escalation).

## What to expect

This is a best-effort, single-maintainer response:

- **Acknowledgement**: within a few days of the report.
- **Fix or mitigation**: no fixed SLA. Timeline depends on severity and complexity -
  a confirmed, high-severity issue with a clear fix is prioritized over everything
  else in the backlog; a low-severity or hard-to-reproduce report may take longer.
- **Disclosure**: coordinated with the reporter. A fix ships first (as a patch
  release of the affected module(s) plus a [CHANGELOG.md](CHANGELOG.md) entry and,
  for anything a consumer should actively act on, a GitHub Security Advisory), with
  public details following once a fix is available.

## Supported versions

Every module in this repo is still `v0.x` (see the [Stability](README.md#stability)
section of the README - the public API can still change between minor versions).
Given that, only **the latest published version of each affected module** receives
security patches; there is no backport policy to older `v0.x` releases. Once (if)
a module reaches `v1.0.0`, this section will be revisited.

## Scope

This policy covers the code in this repository. It does not cover:

- Vulnerabilities in a vendor SDK a module wraps (e.g. a CVE in `go-redis` itself,
  not in `integrations/redis`) - report those upstream. Argos does bump vendored
  dependencies promptly once a fix is available; see `go.sum`/`go.mod` history and
  [CHANGELOG.md](CHANGELOG.md) for examples.
- Vulnerabilities in the `samples/*` or `examples/*` demo applications - these are
  reference code, not published/versioned modules (see the release pipeline's own
  guard against tagging them), and are not held to the same hardening bar as the
  library itself.
