# Git Commit & Review Policy (Strict)

## 1. Commit Size Constraint

- **Rule:** A single commit MUST NOT exceed 500 lines of **diff** (added + deleted lines combined).
- **Exclusions:** Ignore changes in auto-generated files (e.g., `zz_generated.*`, `vendor/`, `go.sum`, or `static/` assets).
- **Action:** If a proposed change exceeds this limit, STOP and instruct the user to split the work into smaller, logical, atomic commits.

## 2. Review Logic

- **Atomic Changes:** Refactors, documentation, and new features must be in separate commits.
- **Rejection:** Do not suggest or accept "mega-commits." Any change over 500 lines is a violation of these instructions.

## 3. Feedback Style

- **Focus:** Prioritize checking line counts and commit atomicity above all else.
- **Tone:** Be direct and concise. If a rule is broken, state which section (e.g., "Violation of Rule 1.1") was triggered.
