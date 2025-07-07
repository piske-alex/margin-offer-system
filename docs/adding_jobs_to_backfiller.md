# Adding Jobs to Continuous Backfiller

This guide explains the different ways to add jobs to the continuous backfiller service.

## Overview

The backfiller supports multiple methods for adding jobs:

1. **Default Jobs** - Hardcoded in `BackfillLatest()` method
2. **Configuration Files** - JSON-based job configuration
3. **Environment Variables** - Dynamic job configuration
4. **Programmatic Registration** - Code-based job registration
5. **Scheduled Jobs** - Cron-based scheduling

## Method 1: Default Jobs (BackfillLatest)

The `BackfillLatest()` method contains the default jobs that run every 6 hours:

```go
func (b *Backfiller) BackfillLatest(ctx context.Context) error {
    // Create backfill requests for different sources
    requests := []*types.BackfillRequest{
        {
            Source:    "chain",
            StartTime: startTime,
            EndTime:   endTime,
            BatchSize: b.defaultBatchSize,
            ChainParams: &types.ChainBackfillParams{
                Programs: []string{"marginfi", "leverage"},
                Network:  "mainnet",
            },
        },
        {
            Source:    "glacier",
            StartTime: startTime,
            EndTime:   endTime,
            BatchSize: b.defaultBatchSize / 2,
            GlacierParams: &types.GlacierBackfillParams{
                VaultName: "margin-offers-archive",
                Retrieval: true,
            },
        },
        // Additional jobs can be added here
    }
}
```

**To add jobs here:**
1. Edit `backfillers/backfiller.go`
2. Add new job definitions to the `requests` slice
3. Rebuild the backfiller

## Method 2: Configuration Files

### JSON Configuration Format

Create a JSON file (e.g., `config/backfiller-jobs.json`):

```json
{
  "default_interval": "6h",
  "jobs": [
    {
      "name": "chain-backfill",
      "description": "Default blockchain backfill",
      "enabled": true,
      "request": {
        "source": "chain",
        "batch_size": 1000,
        "chain_params": {
          "programs": ["marginfi", "leverage"],
          "network": "mainnet"
        }
      },
      "max_retries": 3,
      "timeout": "1h"
    },
    {
      "name": "daily-sync",
      "description": "Daily full sync",
      "enabled": true,
      "schedule": "0 2 * * *",
      "request": {
        "source": "chain",
        "batch_size": 2000,
        "chain_params": {
          "programs": ["CRSeeBqjDnm3UPefJ9gxrtngTsnQRhEJiTA345Q83X3v"],
          "network": "mainnet"
        }
      }
    }
  ]
}
```

### Loading Configuration

```bash
# Run backfiller with config file
./bin/backfillers --config config/backfiller-jobs.json
```

Or programmatically:

```go
backfiller := backfillers.NewBackfiller(storeClient, chainClient, logger, metrics)
err := backfiller.LoadJobsFromFile("config/backfiller-jobs.json")
```

## Method 3: Environment Variables

Set the `BACKFILL_JOBS` environment variable with JSON job configuration:

```bash
export BACKFILL_JOBS='[
  {
    "name": "env-job",
    "enabled": true,
    "request": {
      "source": "chain",
      "batch_size": 1000,
      "chain_params": {
        "programs": ["marginfi"],
        "network": "mainnet"
      }
    }
  }
]'

./bin/backfillers
```

Or programmatically:

```go
backfiller := backfillers.NewBackfiller(storeClient, chainClient, logger, metrics)
err := backfiller.LoadJobsFromEnv()
```

## Method 4: Programmatic Registration

### Register Custom Jobs

```go
// Register a custom job that runs with BackfillLatest
customJob := &types.BackfillRequest{
    Source:    "chain",
    BatchSize: 1500,
    ChainParams: &types.ChainBackfillParams{
        Programs: []string{"marginfi"},
        Network:  "mainnet",
    },
}

err := backfiller.RegisterCustomJob(customJob)
```

### Register Scheduled Jobs

```go
// Register a job that runs on a specific schedule
scheduledJob := &types.BackfillRequest{
    Source:    "chain",
    BatchSize: 2000,
    ChainParams: &types.ChainBackfillParams{
        Programs: []string{"leverage"},
        Network:  "mainnet",
    },
}

// Run every day at 2 AM
err := backfiller.RegisterScheduledJob("daily-sync", "0 2 * * *", scheduledJob)
```

## Method 5: Cron Scheduling

The backfiller supports cron expressions for scheduled jobs:

### Common Cron Patterns

```bash
# Every 15 minutes
"*/15 * * * *"

# Every hour
"0 * * * *"

# Every 4 hours
"0 */4 * * *"

# Every day at 2 AM
"0 2 * * *"

# Every Sunday at 1 AM
"0 1 * * 0"

# Every Monday, Wednesday, Friday at 3 AM
"0 3 * * 1,3,5"
```

### Example Scheduled Jobs

```go
// Quick sync every 15 minutes
quickSyncJob := &types.BackfillRequest{
    Source:    "chain",
    BatchSize: 300,
    ChainParams: &types.ChainBackfillParams{
        Programs: []string{"leverage"},
        Network:  "mainnet",
    },
}
backfiller.RegisterScheduledJob("quick-sync", "*/15 * * * *", quickSyncJob)

// Medium sync every 4 hours
mediumSyncJob := &types.BackfillRequest{
    Source:    "chain",
    BatchSize: 1000,
    ChainParams: &types.ChainBackfillParams{
        Programs: []string{"marginfi", "leverage"},
        Network:  "mainnet",
    },
}
backfiller.RegisterScheduledJob("medium-sync", "0 */4 * * *", mediumSyncJob)

// Full sync every Sunday at 1 AM
fullSyncJob := &types.BackfillRequest{
    Source:    "chain",
    BatchSize: 2500,
    ChainParams: &types.ChainBackfillParams{
        Programs: []string{"marginfi", "leverage", "CRSeeBqjDnm3UPefJ9gxrtngTsnQRhEJiTA345Q83X3v"},
        Network:  "mainnet",
    },
}
backfiller.RegisterScheduledJob("full-sync", "0 1 * * 0", fullSyncJob)
```

## Job Types

### Chain Jobs

```go
chainJob := &types.BackfillRequest{
    Source:    "chain",
    BatchSize: 1000,
    ChainParams: &types.ChainBackfillParams{
        StartBlock: 0,           // Optional: specific block range
        EndBlock:   1000000,     // Optional: specific block range
        Programs:   []string{"marginfi", "leverage"},
        Network:    "mainnet",
    },
}
```

### Glacier Jobs

```go
glacierJob := &types.BackfillRequest{
    Source:    "glacier",
    BatchSize: 500,
    GlacierParams: &types.GlacierBackfillParams{
        VaultName: "margin-offers-archive",
        ArchiveID: "optional-archive-id",
        JobID:     "optional-job-id",
        Retrieval: true,
    },
}
```

### Off-Chain Jobs

```go
offChainJob := &types.BackfillRequest{
    Source:    "off-chain",
    BatchSize: 1000,
    OffChainParams: &types.OffChainBackfillParams{
        APIEndpoint: "https://api.example.com/margin-offers",
        Headers: map[string]string{
            "Authorization": "Bearer ${API_TOKEN}",
            "Content-Type":  "application/json",
        },
        QueryParams: map[string]string{
            "limit": "1000",
        },
        AuthToken: "optional-auth-token",
    },
}
```

## Complete Example

See `examples/backfiller_jobs_demo.go` for a complete example showing all methods:

```bash
# Build the example
go build -o bin/backfiller-jobs-demo examples/backfiller_jobs_demo.go

# Run the example
./bin/backfiller-jobs-demo
```

## Command Line Usage

```bash
# Run with default jobs only
./bin/backfillers

# Run with configuration file
./bin/backfillers --config config/backfiller-jobs.json

# Run with environment variables
BACKFILL_JOBS='[...]' ./bin/backfillers

# Run one-time backfill
./bin/backfillers --run-once --source chain --hours 24
```

## Best Practices

1. **Use Configuration Files** for production deployments
2. **Use Environment Variables** for dynamic configuration
3. **Use Programmatic Registration** for development and testing
4. **Use Scheduled Jobs** for time-based operations
5. **Monitor Job Performance** using the status and metrics APIs
6. **Set Appropriate Batch Sizes** based on your data volume
7. **Use Retry Logic** for transient failures
8. **Set Timeouts** to prevent hanging jobs

## Monitoring Jobs

```go
// Get overall status
status := backfiller.GetStatus()
fmt.Printf("Active jobs: %d\n", len(status.ActiveJobs))
fmt.Printf("Completed jobs: %d\n", status.CompletedJobs)
fmt.Printf("Failed jobs: %d\n", status.FailedJobs)

// Get specific job progress
progress := backfiller.GetProgress()
fmt.Printf("Current job: %s\n", progress.JobID)
fmt.Printf("Progress: %.2f%%\n", progress.Progress*100)
``` 