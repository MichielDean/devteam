# Deep Review Extension (Mandatory for Standard/RFC Implementations)

When this extension is loaded (automatically for features implementing standards, RFCs, or protocols), the pipeline performs deep spec-compliance verification beyond acceptance criteria.

## Why This Exists

Acceptance criteria can pass while the implementation violates the standard. PR #32 had 226 passing tests and 11 correctness/security bugs found by expert review. The tests tested the developer's interpretation, not the standard's requirements. Deep review closes this gap by forcing the pipeline to verify the implementation against the standard itself — not against the developer's reading of it.

## When to Load

Load this extension when the feature implements:
- An RFC (9421, 7519, 7517, 6749, etc.)
- A protocol (HTTP signing, OAuth, JWT, JWKS, webhooks)
- A conformance suite with test vectors
- A security standard (FIPS, NIST, OWASP)
- Any specification with negative test cases

The PM identifies this during source discovery and flags the feature for deep review.

## Phase Additions

### Inception (PM) — Deep Source Reading

The PM doesn't just list the RFC — reads the relevant sections and extracts constraints:

1. **Read the RFC sections that govern the feature's behavior** — not just the introduction
2. **For each section, extract verifiable constraints** — "the verifier MUST reject malformed Signature-Input" becomes CON-XXX
3. **Map negative test vectors to constraints** — each vector is a constraint with a specific expected rejection
4. **Identify error taxonomies** — the standard's error codes, not invented ones
5. **Identify cross-component requirements** — if the standard says "all signers MUST," that applies to every signer implementation

### Planning (Architect) — Constraint-Driven Design

The architect produces:
1. **Constraint verification map** — every constraint traces to a design decision, component, and test
2. **Cross-component consistency matrix** — every shared value verified across all producers and consumers
3. **Negative case design** — for every negative vector, how the implementation rejects it
4. **Multi-component constraint application** — if a constraint applies to N components, all N are listed

### Review (Reviewer) — Execution Path Tracing

The reviewer doesn't just check acceptance criteria — traces execution paths:

1. **For each constraint, trace the data path from input to output**
2. **Follow every branch** — happy path, every error path, every edge case
3. **Verify the constraint holds on every path** — not just the obvious one
4. **Check all components** — if a constraint applies to 4 providers, verify all 4
5. **Check language-specific footguns** — modulo, nil maps, negative repeat, overflow
6. **Verify negative test vectors are rejected** — not just that positive cases work

### Testing (Tester) — Conformance Testing

The tester writes conformance tests against the standard's test vectors:

1. **Level 0 conformance tests** — every negative vector gets a test
2. **Multi-component tests** — if a constraint applies to N components, test all N
3. **Language footgun tests** — test edge cases that compile but produce wrong behavior
4. **Adversarial probing** — try to break each constraint, not just verify the happy path

## Deep Review Checklist

Before a feature implementing a standard can pass the review and testing gates:

- [ ] Every constraint from the register has a design decision
- [ ] Every constraint has a test that would fail if violated
- [ ] Every negative test vector has a conformance test
- [ ] Cross-component consistency verified for all shared values
- [ ] Execution paths traced for all constraints
- [ ] Language-specific footguns checked
- [ ] Multi-component constraints verified across ALL components
- [ ] Error codes match the standard's taxonomy
- [ ] No parse failures throw exceptions where the standard requires rejection results
- [ ] No silent acceptance of malformed input