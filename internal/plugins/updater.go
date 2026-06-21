package plugins

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MichielDean/devteam/internal/config"
)

const cacheDir = "plugins"

type Updater struct {
	config  *config.Config
	baseDir string
	client  *http.Client
}

func NewUpdater(cfg *config.Config, baseDir string) *Updater {
	return &Updater{
		config:  cfg,
		baseDir: baseDir,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (u *Updater) UpdateAll(ctx context.Context) error {
	if u.config.Plugins == nil {
		fmt.Println("No plugins configured.")
		return nil
	}
	for name, plugin := range u.config.Plugins {
		if err := u.Update(ctx, name, &plugin); err != nil {
			return fmt.Errorf("updating plugin %s: %w", name, err)
		}
	}
	return nil
}

func (u *Updater) Update(ctx context.Context, name string, plugin *config.PluginConfig) error {
	rulesURL := resolveRulesURL(plugin)
	if rulesURL == "" {
		return fmt.Errorf("plugin %s: cannot resolve rules URL from source %s", name, plugin.Source)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rulesURL, nil)
	if err != nil {
		return fmt.Errorf("creating request for %s: %w", rulesURL, err)
	}

	resp, err := u.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetching %s: %w", rulesURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetching %s: HTTP %d", rulesURL, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response from %s: %w", rulesURL, err)
	}

	pluginDir := filepath.Join(u.baseDir, cacheDir, name)
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return fmt.Errorf("creating plugin dir %s: %w", pluginDir, err)
	}

	rulesPath := filepath.Join(pluginDir, "rules.md")
	if err := os.WriteFile(rulesPath, body, 0644); err != nil {
		return fmt.Errorf("writing rules to %s: %w", rulesPath, err)
	}

	metadata := fmt.Sprintf("name: %s\nsource: %s\nmode: %s\nupdated: %s\n", name, plugin.Source, plugin.Mode, time.Now().Format(time.RFC3339))
	metaPath := filepath.Join(pluginDir, "plugin.yaml")
	if err := os.WriteFile(metaPath, []byte(metadata), 0644); err != nil {
		return fmt.Errorf("writing metadata to %s: %w", metaPath, err)
	}

	log.Printf("plugin %s: updated from %s (%d bytes)", name, rulesURL, len(body))
	return nil
}

func resolveRulesURL(plugin *config.PluginConfig) string {
	source := strings.TrimSuffix(plugin.Source, "/")
	if strings.Contains(source, "github.com") {
		parts := strings.Split(strings.TrimPrefix(source, "https://"), "/")
		if len(parts) >= 3 {
			org := parts[1]
			repo := parts[2]
			return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/skills/ponytail/SKILL.md", org, repo)
		}
	}
	return ""
}

func LoadCachedRules(baseDir string, pluginName string) (string, error) {
	rulesPath := filepath.Join(baseDir, cacheDir, pluginName, "rules.md")
	data, err := os.ReadFile(rulesPath)
	if err != nil {
		return "", fmt.Errorf("plugin %s not installed (run 'devteam plugin update'): %w", pluginName, err)
	}
	return string(data), nil
}

func IsInstalled(baseDir string, pluginName string) bool {
	rulesPath := filepath.Join(baseDir, cacheDir, pluginName, "rules.md")
	_, err := os.Stat(rulesPath)
	return err == nil
}