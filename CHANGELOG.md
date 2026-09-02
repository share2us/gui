# Changelog

All notable user-facing changes to the Share2Us desktop app.

The release workflow reads the `[Unreleased]` section below into the GitHub
release notes, and **refuses to cut a stable release while it is empty** — so a
build cannot reach users without saying what changed. On release, move the
section under a new version heading. HTML comments do not count as content, so
a section holding only a note still blocks a stable release.

`[Unreleased]` is genuinely empty right now, and that is correct: the last
release (`v20260801223900`, 2026-08-01) already contains everything on `main`
except the release-workflow change itself, which is not user-facing. The
broadcast / home-feed redesign, transfer resume and the LAN security pass all
shipped before that tag — they were released without written notes, which is the
gap this file closes going forward, not a backlog of unreleased work.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versions are UTC build timestamps (`20260902114433`), not semver.

## [Unreleased]

## Earlier

Releases before this changelog existed. See the GitHub releases list for the
build history.
