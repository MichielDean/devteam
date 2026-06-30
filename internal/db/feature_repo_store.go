package db

import "fmt"

// FeatureRepoRow is a prepared implementation repo worktree record.
type FeatureRepoRow struct {
	FeatureID string `json:"feature_id"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	Dir       string `json:"dir"`
	Branch    string `json:"branch"`
}

// SaveFeatureRepo inserts or replaces a prepared repo for a feature.
func (db *DB) SaveFeatureRepo(featureID, name, url, dir, branch string) error {
	_, err := db.Exec(
		`INSERT INTO feature_repos (feature_id, name, url, dir, branch)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(feature_id, name) DO UPDATE SET url = excluded.url, dir = excluded.dir, branch = excluded.branch`,
		featureID, name, url, dir, branch,
	)
	if err != nil {
		return fmt.Errorf("saving feature repo %s/%s: %w", featureID, name, err)
	}
	return nil
}

// GetFeatureRepos returns all prepared repos for a feature.
func (db *DB) GetFeatureRepos(featureID string) ([]FeatureRepoRow, error) {
	rows, err := db.Query(
		`SELECT feature_id, name, url, dir, branch FROM feature_repos WHERE feature_id = ?`,
		featureID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting feature repos for %s: %w", featureID, err)
	}
	defer rows.Close()

	var repos []FeatureRepoRow
	for rows.Next() {
		var r FeatureRepoRow
		if err := rows.Scan(&r.FeatureID, &r.Name, &r.URL, &r.Dir, &r.Branch); err != nil {
			return nil, fmt.Errorf("scanning feature repo: %w", err)
		}
		repos = append(repos, r)
	}
	return repos, nil
}

// DeleteFeatureRepos removes all prepared repo records for a feature.
func (db *DB) DeleteFeatureRepos(featureID string) error {
	_, err := db.Exec(`DELETE FROM feature_repos WHERE feature_id = ?`, featureID)
	if err != nil {
		return fmt.Errorf("deleting feature repos for %s: %w", featureID, err)
	}
	return nil
}