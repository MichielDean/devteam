package db

import (
	"fmt"
	"time"
)

// ArtifactRow represents a spec artifact stored in the database.
type ArtifactRow struct {
	ID           int64     `json:"id"`
	FeatureID    string    `json:"feature_id"`
	ArtifactType string    `json:"artifact_type"`
	Content      string    `json:"content"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// SaveArtifact stores or updates a spec artifact in the database.
func (db *DB) SaveArtifact(featureID, artifactType, content string) error {
	now := time.Now().UTC()
	_, err := db.Exec(
		`INSERT INTO spec_artifacts (feature_id, artifact_type, content, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(feature_id, artifact_type) DO UPDATE SET content = excluded.content, updated_at = excluded.updated_at`,
		featureID, artifactType, content, now, now,
	)
	if err != nil {
		return fmt.Errorf("saving artifact: %w", err)
	}
	return nil
}

// GetArtifact retrieves a spec artifact from the database.
func (db *DB) GetArtifact(featureID, artifactType string) (*ArtifactRow, error) {
	row := db.QueryRow(
		`SELECT id, feature_id, artifact_type, content, created_at, updated_at
		 FROM spec_artifacts WHERE feature_id = ? AND artifact_type = ?`,
		featureID, artifactType,
	)

	var a ArtifactRow
	err := row.Scan(&a.ID, &a.FeatureID, &a.ArtifactType, &a.Content, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("artifact not found: %w", err)
	}
	return &a, nil
}

// ListArtifacts retrieves all spec artifacts for a feature.
func (db *DB) ListArtifacts(featureID string) ([]ArtifactRow, error) {
	rows, err := db.Query(
		`SELECT id, feature_id, artifact_type, content, created_at, updated_at
		 FROM spec_artifacts WHERE feature_id = ? ORDER BY created_at ASC`,
		featureID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing artifacts: %w", err)
	}
	defer rows.Close()

	var artifacts []ArtifactRow
	for rows.Next() {
		var a ArtifactRow
		if err := rows.Scan(&a.ID, &a.FeatureID, &a.ArtifactType, &a.Content, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning artifact: %w", err)
		}
		artifacts = append(artifacts, a)
	}
	return artifacts, nil
}

// DeleteArtifact removes a spec artifact from the database.
func (db *DB) DeleteArtifact(featureID, artifactType string) error {
	_, err := db.Exec(
		`DELETE FROM spec_artifacts WHERE feature_id = ? AND artifact_type = ?`,
		featureID, artifactType,
	)
	if err != nil {
		return fmt.Errorf("deleting artifact: %w", err)
	}
	return nil
}