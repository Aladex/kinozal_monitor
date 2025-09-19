package models

import (
	"fmt"
	"kinozaltv_monitor/config"
	logger "kinozaltv_monitor/logging"
	"strings"
)

// TrackerManager manages multiple torrent trackers
type TrackerManager struct {
	trackers map[string]TorrentTracker
	log      *logger.Logger
}

// NewTrackerManager creates a new tracker manager with initialized trackers
func NewTrackerManager(globalConfig *config.AppConfig) *TrackerManager {
	manager := &TrackerManager{
		trackers: make(map[string]TorrentTracker),
		log:      logger.New("tracker_manager"),
	}

	// Initialize Kinozal tracker if credentials are provided
	if globalConfig.KinozalUsername != "" && globalConfig.KinozalPassword != "" {
		kinozalTracker := NewKinozalTracker(globalConfig)
		err := kinozalTracker.Login()
		if err != nil {
			manager.log.Error("kinozal_init", "Error while logging in to Kinozal", map[string]string{"error": err.Error()})
		} else {
			manager.trackers["kinozal"] = kinozalTracker
			manager.log.Info("kinozal_init", "Kinozal user logged in successfully", nil)
		}
	}

	// Initialize RuTracker tracker if credentials are provided
	if globalConfig.RtUsername != "" && globalConfig.RtPassword != "" {
		rutrackerTracker := NewRuTrackerTracker(globalConfig)
		err := rutrackerTracker.Login()
		if err != nil {
			manager.log.Error("rutracker_init", "Error while logging in to RuTracker", map[string]string{"error": err.Error()})
		} else {
			manager.trackers["rutracker"] = rutrackerTracker
			manager.log.Info("rutracker_init", "RuTracker user logged in successfully", nil)
		}
	}

	return manager
}

// GetTracker returns a tracker by name
func (tm *TrackerManager) GetTracker(name string) (TorrentTracker, error) {
	tracker, exists := tm.trackers[strings.ToLower(name)]
	if !exists {
		return nil, fmt.Errorf("tracker %s not found or not initialized", name)
	}
	return tracker, nil
}

// GetTrackerByURL determines the appropriate tracker based on the URL
func (tm *TrackerManager) GetTrackerByURL(url string) (TorrentTracker, error) {
	url = strings.ToLower(url)

	if strings.Contains(url, "kinozal.tv") {
		return tm.GetTracker("kinozal")
	}

	if strings.Contains(url, "rutracker.org") {
		return tm.GetTracker("rutracker")
	}

	return nil, fmt.Errorf("no suitable tracker found for URL: %s", url)
}

// GetAvailableTrackers returns a list of available tracker names
func (tm *TrackerManager) GetAvailableTrackers() []string {
	var trackers []string
	for name := range tm.trackers {
		trackers = append(trackers, name)
	}
	return trackers
}

// GetTorrentHash retrieves torrent hash using the appropriate tracker based on URL
func (tm *TrackerManager) GetTorrentHash(url string) (Torrent, error) {
	tracker, err := tm.GetTrackerByURL(url)
	if err != nil {
		return Torrent{}, err
	}
	return tracker.GetTorrentHash(url)
}

// DownloadTorrentFile downloads torrent file using the appropriate tracker based on URL
func (tm *TrackerManager) DownloadTorrentFile(url string) ([]byte, error) {
	tracker, err := tm.GetTrackerByURL(url)
	if err != nil {
		return nil, err
	}
	return tracker.DownloadTorrentFile(url)
}

// GetTitleFromUrl gets title using the appropriate tracker based on URL
func (tm *TrackerManager) GetTitleFromUrl(url string) (string, error) {
	tracker, err := tm.GetTrackerByURL(url)
	if err != nil {
		return "", err
	}
	return tracker.GetTitleFromUrl(url)
}

// Global tracker manager instance
var GlobalTrackerManager *TrackerManager

// InitializeTrackers initializes the global tracker manager
func InitializeTrackers(globalConfig *config.AppConfig) {
	GlobalTrackerManager = NewTrackerManager(globalConfig)
}
