# Decisions

This directory is for writing down decisions we make, so that future
teammates (and future us) can understand the "why" behind the code.

It's inspired by the Architecture Decision Record (ADR)
practice described at
<https://github.com/joelparkerhenderson/architecture-decision-record>,
but it isn't limited to architecture. Schema, tools, pipelines, datastructures, services, "why we picked library X", all of
it fits here and can share the same shape.

## When to write one

Write a decision down when you'd want a teammate joining in six
months to know *why* something is the way it is. Skip it when the
decision is trivial, self-contained, or already obvious from the code.

## Guidelines

Everything below is a suggestion to keep entries consistent enough
to be browsable. The goal is useful communication, not paperwork.

### Naming

A name like `NNNN-short-imperative-phrase.md` works well:

- Zero-padded number so files sort nicely.
- Finish the sentence "We are making a decision to ...". (Guideline only)

### Shape

Most entries will benefit from something like:

```markdown
# NNNN. Title

Date: YYYY-MM-DD

## Context

Why are we making this decision? What forces are at play?

## Decision

What we are going to do.

## Consequences

What becomes easier or harder. Follow-ups. Things to revisit.
```

### Living documents

In theory a decision record is immutable. In practice, small updates
are fine and often useful, for example, a new piece of context, a
correction, or a note about how the decision played out in the real
world. When you add information after the fact, put a dated note next
to it so readers know when it arrived and which parts reflect the
original decision vs. later learnings.

If a decision fundamentally changes, prefer writing a new record that
links to and supersedes the old one, rather than rewriting history.

### Lifecycle

Roughly:

1. Draft a record and open a PR.
2. Discuss on the PR. When the team is happy, merge.
3. If a later record replaces this one, add a Status section (if it doesn't already exist), update the status to
   `Superseded by [NNNN](...)`.
