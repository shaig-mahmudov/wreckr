# ADR-013: Add Visual Scenario Builder Alongside JSON/YAML Editing

## Status
Accepted

## Context
Non-technical users, QA engineers, and developers wanting quick scenario mockups need an intuitive, visual way to construct HTTP paths, headers, and traffic profiles without syntax errors or manual JSON editing. However, power users still need the full flexibility, editing speed, and expressiveness of raw configuration text editing.

## Decision
Implement a visual, form-based scenario builder as the default interface while retaining the raw JSON/YAML text editor as a toggle.

## Consequences
- **Expected:** Users can construct complex HTTP scenarios visually with instant feedback while retaining full fidelity and serialization control.
- **Result (Observed):** Next.js dashboard features a visual builder for metadata, targets, traffic profiles, and requests list. Users can toggle to JSON/YAML mode seamlessly. If invalid formatting is input, we prevent visual mode switching and show a validation error.

## Alternatives Considered

### Alternative 1: Replace raw JSON/YAML editor entirely with form inputs
- **Expected if chosen:** The UI would be simpler, but developers could not copy-paste complex config files or quickly customize details outside the form's structured bounds.

### Alternative 2: Keep only raw text editor and add auto-complete/LSP tooling
- **Expected if chosen:** Helping users with schema definition would be possible, but it wouldn't eliminate the friction of learning the JSON/YAML structure, and errors would still be common.
