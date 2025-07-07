package backfillers

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/piske-alex/margin-offer-system/types"
)

// JobConfig represents a job configuration
type JobConfig struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Enabled     bool                   `json:"enabled"`
	Schedule    string                 `json:"schedule,omitempty"` // Cron expression for scheduled jobs
	Request     *types.BackfillRequest `json:"request"`
	MaxRetries  int32                  `json:"max_retries"`
	Timeout     time.Duration          `json:"timeout"`
}

// JobConfigFile represents the structure of a job configuration file
type JobConfigFile struct {
	DefaultInterval time.Duration `json:"default_interval"`
	Jobs            []JobConfig   `json:"jobs"`
}

// LoadJobsFromFile loads job configurations from a JSON file
func (b *Backfiller) LoadJobsFromFile(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var config JobConfigFile
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Update default interval if specified
	if config.DefaultInterval > 0 {
		b.backfillInterval = config.DefaultInterval
	}

	// Register each job
	for _, jobConfig := range config.Jobs {
		if !jobConfig.Enabled {
			b.logger.Info("Skipping disabled job", "name", jobConfig.Name)
			continue
		}

		if jobConfig.Schedule != "" {
			// Register as scheduled job
			if err := b.RegisterScheduledJob(jobConfig.Name, jobConfig.Schedule, jobConfig.Request); err != nil {
				b.logger.Error("Failed to register scheduled job", "name", jobConfig.Name, "error", err)
				continue
			}
		} else {
			// Register as custom job (will be included in BackfillLatest)
			if err := b.RegisterCustomJob(jobConfig.Request); err != nil {
				b.logger.Error("Failed to register custom job", "name", jobConfig.Name, "error", err)
				continue
			}
		}

		b.logger.Info("Registered job from config", "name", jobConfig.Name, "type", jobConfig.Schedule)
	}

	return nil
}

// LoadJobsFromEnv loads job configurations from environment variables
func (b *Backfiller) LoadJobsFromEnv() error {
	// Example: BACKFILL_JOBS='[{"name":"env-job","enabled":true,"request":{"source":"chain","batch_size":1000}}]'
	jobsJSON := os.Getenv("BACKFILL_JOBS")
	if jobsJSON == "" {
		return nil // No jobs configured via env
	}

	var jobs []JobConfig
	if err := json.Unmarshal([]byte(jobsJSON), &jobs); err != nil {
		return fmt.Errorf("failed to parse BACKFILL_JOBS env var: %w", err)
	}

	for _, job := range jobs {
		if !job.Enabled {
			continue
		}

		if job.Schedule != "" {
			if err := b.RegisterScheduledJob(job.Name, job.Schedule, job.Request); err != nil {
				b.logger.Error("Failed to register scheduled job from env", "name", job.Name, "error", err)
			}
		} else {
			if err := b.RegisterCustomJob(job.Request); err != nil {
				b.logger.Error("Failed to register custom job from env", "name", job.Name, "error", err)
			}
		}
	}

	return nil
}

// GetDefaultJobConfig returns a default job configuration
func GetDefaultJobConfig() JobConfigFile {
	return JobConfigFile{
		DefaultInterval: 6 * time.Hour,
		Jobs: []JobConfig{
			{
				Name:        "chain-backfill",
				Description: "Default blockchain backfill",
				Enabled:     true,
				Request: &types.BackfillRequest{
					Source:    "chain",
					BatchSize: 1000,
					ChainParams: &types.ChainBackfillParams{
						Programs: []string{"marginfi", "leverage"},
						Network:  "mainnet",
					},
				},
				MaxRetries: 3,
				Timeout:    time.Hour,
			},
			{
				Name:        "glacier-backfill",
				Description: "Default Glacier backfill",
				Enabled:     true,
				Request: &types.BackfillRequest{
					Source:    "glacier",
					BatchSize: 500,
					GlacierParams: &types.GlacierBackfillParams{
						VaultName: "margin-offers-archive",
						Retrieval: true,
					},
				},
				MaxRetries: 3,
				Timeout:    time.Hour,
			},
			{
				Name:        "daily-leverage-sync",
				Description: "Daily leverage program sync",
				Enabled:     true,
				Schedule:    "0 2 * * *", // 2 AM daily
				Request: &types.BackfillRequest{
					Source:    "chain",
					BatchSize: 1000,
					ChainParams: &types.ChainBackfillParams{
						Programs: []string{"CRSeeBqjDnm3UPefJ9gxrtngTsnQRhEJiTA345Q83X3v"},
						Network:  "mainnet",
					},
				},
				MaxRetries: 3,
				Timeout:    time.Hour,
			},
		},
	}
}
