# Delivery Phase Rules

## Purpose

Ship and document. Ensure documentation matches the spec and the release is coordinated.

## Ops Responsibilities

1. **Document**: Write documentation using terminology from the spec
2. **Coordinate**: Manage cross-repo release ordering
3. **Verify Docs**: Ensure documentation matches spec terminology and acceptance criteria
4. **Release**: Build, tag, and deploy in the correct order

## Step 1: Documentation

### API Documentation

For every endpoint in the plan, produce documentation:

```markdown
### [METHOD] [path]

**Purpose**: [what it does, matching spec terminology]

**Request**:
- `field` (type, required/optional): description

**Response 200**:
- `field` (type): description

**Response 400**:
- `error` (string): error code
- `details` (string): human-readable message

**Response 404**:
- `error`: "not_found"
- `details`: "[resource] not found"
```

### User-Facing Documentation

For every user story in the spec, produce documentation that:
- Uses the same terminology defined in spec.md
- References user stories from the spec
- Includes examples for common workflows
- Documents error messages and their meanings

### Changelog

```markdown
## [version] - [date]

### Added
- [feature description] (spec #NNN)

### Changed
- [change description] (spec #NNN)

### Fixed
- [fix description] (spec #NNN)
```

Every changelog entry MUST reference the spec number.

## Step 2: Cross-Repo Release Coordination

### Release Order

When a feature spans repos, determine the correct release order:

1. **Shared libraries/APIs first**: Repos that other repos depend on
2. **Consumers second**: Repos that import the shared libraries
3. **Frontend last**: UI repos that consume the APIs

### Release Order Template

```markdown
## Release Order

1. [shared-library-repo] - v[version]
   - Reason: Other repos depend on this
   - Breaking changes: [none / list]
   - Migration required: [yes/no]

2. [api-repo] - v[version]
   - Reason: Depends on shared-library v[version]
   - Breaking changes: [none / list]

3. [frontend-repo] - v[version]
   - Reason: Depends on api v[version]
   - Breaking changes: [none / list]
```

### Coordinated Release

For multi-repo releases:
1. Tag all repos with consistent version references
2. Update each repo's dependency pointers
3. Test each repo builds against the new dependencies
4. Release in dependency order (shared → consumers → frontend)
5. Update each repo's `.devteam/` pointer to mark the spec as delivered

## Step 3: Build and Deployment

### Build Verification

Before marking delivery as complete:

1. **Build the binary** — `go build -o ~/go/bin/devteam ./cmd/devteam/`
2. **Run the full test suite** — `go test ./...`
3. **Verify build succeeds** with no warnings that weren't there before

### Deployment Verification

1. **Start the service** — verify it starts without panicking
2. **Hit the endpoints** — verify the API responds correctly
3. **Load the UI** — verify the frontend renders without console errors
4. **Run smoke tests** — verify the service passes all smoke tests from the testing phase

If the service doesn't start or the UI doesn't load, delivery is not complete.

### Configuration Verification

1. **Environment variables**: Document all required env vars
2. **Configuration files**: Verify config files are correct
3. **Dependencies**: Verify all dependencies are at correct versions
4. **Database migrations**: If applicable, verify migrations run correctly

## Step 4: Documentation Review

### Terminology Consistency Check

Compare documentation terminology against spec.md:
- Are the same terms used in docs as in the spec?
- Are API endpoint names consistent between docs and implementation?
- Are error messages consistent between docs and implementation?

If the implementation uses different terminology than the spec, either:
- Update the docs to match the spec (preferred), or
- Update the spec to match the implementation (if the spec was wrong)

Do NOT leave terminology mismatches.

### Documentation Completeness Check

For every user story in the spec:
- [ ] Is there documentation for this feature?
- [ ] Does the documentation use spec terminology?
- [ ] Does the documentation cover error scenarios?
- [ ] Does the documentation reference the spec number?

## Quality Gate

The release is ready when:
1. Documentation exists for every user story
2. Documentation uses spec terminology (not code-internal names)
3. Cross-repo release order is documented and followed
4. Release notes reference the spec number
5. Each affected repo builds and deploys successfully
6. The service starts and responds to HTTP requests
7. The frontend loads without console errors
8. All smoke tests from the testing phase still pass
9. Configuration is documented
10. Breaking changes (if any) are documented with migration steps