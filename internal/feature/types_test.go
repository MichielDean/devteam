package feature

import "testing"

func TestArtifactAPIPathToType(t *testing.T) {
	tt := []struct {
		name    string
		in      string
		want    ArtifactType
		wantOK  bool
	}{
		{"empty rejected", "", "", false},
		{"spec alias", "spec", ArtifactSpecMD, true},
		{"acceptance alias", "acceptance", ArtifactAcceptanceMD, true},
		{"repos alias", "repos", ArtifactReposYAML, true},
		{"review_report alias", "review_report", ArtifactReviewReport, true},
		{"stage artifact passthrough", "intent-statement", ArtifactType("intent-statement"), true},
		{"stage artifact passthrough 2", "stakeholder-map", ArtifactType("stakeholder-map"), true},
		{"stage artifact passthrough 3", "scope-definition", ArtifactType("scope-definition"), true},
		{"arbitrary string passthrough", "anything-else", ArtifactType("anything-else"), true},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ArtifactAPIPathToType(tc.in)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if got != tc.want {
				t.Fatalf("type = %q, want %q", got, tc.want)
			}
		})
	}
}