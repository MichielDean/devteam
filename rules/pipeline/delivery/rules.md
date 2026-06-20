# Delivery Phase Rules

## Purpose

Ship and document. Ensure documentation matches the spec and the release is coordinated.

## Ops Responsibilities

1. **Document**: Write documentation using terminology from the spec
2. **Coordinate**: Manage cross-repo release ordering
3. **Verify Docs**: Ensure documentation matches spec terminology and acceptance criteria
4. **Release**: Build, tag, and deploy in the correct order

## Documentation Standards

- Use the same terminology defined in spec.md
- API documentation matches the contracts in plan.md
- User-facing docs reference user stories from the spec
- Changelog entries reference the spec number

## Cross-Repo Release

When a feature spans repos:
1. Release shared libraries/APIs first
2. Release consumers second
3. Tag all repos with consistent version references
4. Update each repo's .devteam/ pointer to mark the spec as delivered

## Deployment Verification

Before marking delivery as complete:
1. **Build the binary** — `go build -o ~/go/bin/devteam ./cmd/devteam/`
2. **Start the service** — verify it starts without panicking
3. **Hit the endpoints** — verify the API responds correctly
4. **Load the UI** — verify the frontend renders without console errors
5. **Run the test suite** — verify all tests pass

If the service doesn't start or the UI doesn't load, delivery is not complete.

## Quality Gate

The release is ready when:
1. Documentation exists for every user story
2. Documentation uses spec terminology (not code-internal names)
3. Cross-repo release order is documented and followed
4. Release notes reference the spec number
5. Each affected repo builds and deploys successfully
6. The service starts and responds to HTTP requests
7. The frontend loads without console errors