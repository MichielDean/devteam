package intake

import (
	"testing"

	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/spec"
)

func TestLooseIdeaIntake(t *testing.T) {
	dir, database := setupTestIntake(t)
	li := NewLooseIdeaIntake(dir)
	li.SetDatabase(database)

	f, err := li.Submit("User Authentication", "We need user auth with GitHub and email login", 2, nil)
	if err != nil {
		t.Fatalf("Submit() error: %v", err)
	}
	if f.ID == "" {
		t.Error("expected non-empty feature ID")
	}
	if f.Status != feature.StatusInProgress {
		t.Errorf("expected status in_progress, got %s", f.Status)
	}
	if f.IntakePath != feature.IntakeLooseIdea {
		t.Errorf("expected intake path loose_idea, got %s", f.IntakePath)
	}

	provider := spec.NewSpecProvider(dir)
	provider.SetDatabase(database)
	if !provider.ArtifactExists(f.ID, feature.ArtifactInputMD) {
		t.Error("expected input.md to exist after intake")
	}

	inputContent, err := provider.ReadArtifact(f.ID, feature.ArtifactInputMD)
	if err != nil {
		t.Fatalf("reading input.md: %v", err)
	}
	if len(inputContent) == 0 {
		t.Error("input.md is empty")
	}
}

func TestLooseIdeaIntakeWithRepos(t *testing.T) {
	dir, database := setupTestIntake(t)
	li := NewLooseIdeaIntake(dir)
	li.SetDatabase(database)

	repos := []feature.RepoRef{
		{Name: "cistern", URL: "git@github.com:MichielDean/cistern.git"},
		{Name: "LLMem", URL: "git@github.com:MichielDean/LLMem.git"},
	}
	f, err := li.Submit("Cross-repo auth", "Auth spanning cistern and LLMem", 1, repos)
	if err != nil {
		t.Fatalf("Submit() error: %v", err)
	}
	if len(f.Repos) != 2 {
		t.Errorf("expected 2 repos, got %d", len(f.Repos))
	}

	provider := spec.NewSpecProvider(dir)
	provider.SetDatabase(database)
	if !provider.ArtifactExists(f.ID, feature.ArtifactReposYAML) {
		t.Error("expected repos.yaml to exist when repos are provided")
	}
}

func TestLooseIdeaInputContent(t *testing.T) {
	dir, database := setupTestIntake(t)
	li := NewLooseIdeaIntake(dir)
	li.SetDatabase(database)

	f, err := li.Submit("Test Feature", "A test description", 2, nil)
	if err != nil {
		t.Fatalf("Submit() error: %v", err)
	}

	provider := spec.NewSpecProvider(dir)
	provider.SetDatabase(database)
	inputContent, err := provider.ReadArtifact(f.ID, feature.ArtifactInputMD)
	if err != nil {
		t.Fatalf("reading input.md: %v", err)
	}
	if len(inputContent) == 0 {
		t.Error("input.md is empty")
	}
}

func TestExternalSpecIntake(t *testing.T) {
	dir, database := setupTestIntake(t)
	es := NewExternalSpecIntake(dir)
	es.SetDatabase(database)

	prd := `# Product Requirements Document

## Overview
This document describes the requirements for a new API rate limiting feature.

## Requirements
- FR-001: Rate limit API requests per user
- FR-002: Configurable rate limits per endpoint
`

	result, err := es.Submit("API Rate Limiting", prd, 2, nil)
	if err != nil {
		t.Fatalf("Submit() error: %v", err)
	}
	if len(result.Features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(result.Features))
	}
	f := result.Features[0]
	if f.IntakePath != feature.IntakeExternalSpec {
		t.Errorf("expected intake path external_spec, got %s", f.IntakePath)
	}

	provider := spec.NewSpecProvider(dir)
	provider.SetDatabase(database)
	if !provider.ArtifactExists(f.ID, feature.ArtifactInputMD) {
		t.Error("expected input.md to exist after external intake")
	}

	inputContent, err := provider.ReadArtifact(f.ID, feature.ArtifactInputMD)
	if err != nil {
		t.Fatalf("reading input.md: %v", err)
	}
	if len(inputContent) == 0 {
		t.Error("input.md is empty")
	}
}

func TestGenerateFeatureID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"User Authentication", "user-authentication"},
		{"API Rate Limiting", "api-rate-limiting"},
		{"Simple Feature", "simple-feature"},
		{"Feature With Numbers 123", "feature-with-numbers-123"},
	}
	for _, tt := range tests {
		got := generateFeatureID(tt.input)
		if got != tt.expected {
			t.Errorf("generateFeatureID(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestExternalSpecParsesSections(t *testing.T) {
	sections := parseSections("# Overview\n## Details\n### Sub-detail\n# Another")
	if len(sections) != 4 {
		t.Errorf("expected 4 sections, got %d", len(sections))
	}
}