# Practices

## Review route
A GitHub pull request against `main`. A maintainer squash-merges it, so `main` gets one commit per pull request and no merge commits. The pull request title becomes that commit's summary.

## Git
Branches: `<issue-number>-<slug>`. Commits and pull request titles: imperative summary, no prefix. Pull request bodies carry `Fixes #<issue-number>`.

## Prerequisites
Docker running for the integration suite.
