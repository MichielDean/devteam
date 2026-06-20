package api

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileWatcher watches .devteam-state.yaml files for changes and
// broadcasts state change events to SSE clients.
// This enables CLI-triggered state changes to be reflected in the UI.
type FileWatcher struct {
	registry     *SSERegistry
	watcher      *fsnotify.Watcher
	mu           sync.Mutex
	stopCh       chan struct{}
	pollInterval time.Duration
	baseDir      string
	lastModTimes map[string]time.Time
}

// NewFileWatcher creates a new FileWatcher that monitors .devteam-state.yaml files
func NewFileWatcher(baseDir string, registry *SSERegistry) *FileWatcher {
	fw := &FileWatcher{
		registry:     registry,
		pollInterval: 2 * time.Second,
		baseDir:      baseDir,
		lastModTimes: make(map[string]time.Time),
		stopCh:       make(chan struct{}),
	}

	// Try to set up fsnotify
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("FileWatcher: fsnotify unavailable, using polling fallback: %v", err)
	} else {
		fw.watcher = watcher
	}

	return fw
}

// Start begins watching for state file changes
func (fw *FileWatcher) Start() {
	if fw.watcher != nil {
		// Watch the specs directory for new subdirectories
		specsDir := filepath.Join(fw.baseDir, "specs")
		if err := fw.watcher.Add(specsDir); err != nil {
			log.Printf("FileWatcher: failed to watch specs dir: %v", err)
		}

		// Also watch each existing feature directory
		entries, err := os.ReadDir(specsDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					featureDir := filepath.Join(specsDir, entry.Name())
					if err := fw.watcher.Add(featureDir); err != nil {
						log.Printf("FileWatcher: failed to watch feature dir %s: %v", featureDir, err)
					}
				}
			}
		}

		go fw.watchLoop()
	} else {
		// Fallback to polling
		go fw.pollLoop()
	}
}

// Stop stops the file watcher
func (fw *FileWatcher) Stop() {
	close(fw.stopCh)
	if fw.watcher != nil {
		fw.watcher.Close()
	}
}

// watchLoop processes fsnotify events
func (fw *FileWatcher) watchLoop() {
	for {
		select {
		case <-fw.stopCh:
			return
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				if strings.HasSuffix(event.Name, ".devteam-state.yaml") {
					fw.handleStateChange(event.Name)
				}
			}
			// Watch newly created feature directories
			if event.Has(fsnotify.Create) {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					if err := fw.watcher.Add(event.Name); err != nil {
						log.Printf("FileWatcher: failed to watch new dir %s: %v", event.Name, err)
					}
				}
			}
		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("FileWatcher error: %v", err)
		}
	}
}

// pollLoop polls for state file changes (fallback when fsnotify is unavailable)
func (fw *FileWatcher) pollLoop() {
	// Initial scan to record baseline modification times
	fw.scanStateFiles()

	ticker := time.NewTicker(fw.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-fw.stopCh:
			return
		case <-ticker.C:
			fw.scanStateFiles()
		}
	}
}

// scanStateFiles checks all .devteam-state.yaml files for changes
func (fw *FileWatcher) scanStateFiles() {
	specsDir := filepath.Join(fw.baseDir, "specs")
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		stateFile := filepath.Join(specsDir, entry.Name(), ".devteam-state.yaml")
		info, err := os.Stat(stateFile)
		if err != nil {
			continue
		}

		fw.mu.Lock()
		lastMod, exists := fw.lastModTimes[stateFile]
		fw.mu.Unlock()

		if exists && info.ModTime().After(lastMod) {
			featureID := entry.Name()
			fw.broadcastStateChange(featureID)
		}

		fw.mu.Lock()
		fw.lastModTimes[stateFile] = info.ModTime()
		fw.mu.Unlock()
	}
}

// handleStateChange processes a state file change detected by fsnotify
func (fw *FileWatcher) handleStateChange(filePath string) {
	// Extract feature ID from path: specs/<featureID>/.devteam-state.yaml
	dir := filepath.Dir(filePath)
	featureID := filepath.Base(dir)
	fw.broadcastStateChange(featureID)

	// Update modification time
	info, err := os.Stat(filePath)
	if err == nil {
		fw.mu.Lock()
		fw.lastModTimes[filePath] = info.ModTime()
		fw.mu.Unlock()
	}
}

// broadcastStateChange sends a state_change event to all SSE clients for the feature
func (fw *FileWatcher) broadcastStateChange(featureID string) {
	fw.registry.Broadcast(featureID, SSEMessage{
		EventType: "state_change",
		Data:      `{"feature_id":"` + featureID + `"}`,
	})
}