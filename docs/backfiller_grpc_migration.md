# Backfiller gRPC Migration

## Overview

The backfiller service has been updated to use a gRPC client instead of a local in-memory store. This change enables the backfiller to communicate with a remote store service, making it suitable for production deployments where the store service runs as a separate service.

## Changes Made

### 1. New gRPC Client (`store/grpc_client.go`)

Created a new `GRPCClient` that implements the `MarginOfferStore` interface and communicates with the store service via gRPC:

- **Connection Management**: Handles gRPC connection establishment and cleanup
- **Health Checks**: Verifies connectivity to the store service
- **Full Interface Implementation**: Implements all methods from the `MarginOfferStore` interface
- **Type Conversion**: Converts between Go types and protobuf messages
- **Error Handling**: Proper error propagation from gRPC calls

### 2. Updated Backfiller Main (`cmd/backfillers/main.go`)

Modified the main function to:

- Accept a `--store-addr` flag for the store service address
- Create a gRPC client instead of a local memory store
- Test connectivity before starting the backfiller
- Gracefully handle connection failures

### 3. No Changes to Backfiller Logic

The core backfiller logic remains unchanged because it was already using the `MarginOfferStore` interface. The worker processes still call:

```go
b.store.BulkOverwrite(ctx, batch)
```

This now uses the gRPC client instead of the local memory store.

## Usage

### Running the Backfiller

```bash
# Start the store service first
./bin/store

# Run the backfiller with gRPC client
./bin/backfillers --store-addr localhost:8080 --run-once

# Run continuous backfiller service
./bin/backfillers --store-addr localhost:8080
```

### Programmatic Usage

```go
// Create gRPC store client
storeClient, err := store.NewGRPCClient("localhost:8080", logger)
if err != nil {
    log.Fatalf("Failed to create store client: %v", err)
}
defer storeClient.Close()

// Test connection
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

if err := storeClient.HealthCheck(ctx); err != nil {
    log.Fatalf("Failed to connect to store service: %v", err)
}

// Create backfiller with gRPC client
backfiller := backfillers.NewBackfiller(storeClient, chainClient, logger, metrics)
```

## Benefits

1. **Separation of Concerns**: Store service runs independently from backfiller
2. **Scalability**: Multiple backfillers can connect to the same store service
3. **Production Ready**: Suitable for containerized deployments
4. **Fault Tolerance**: Store service can be restarted without affecting backfillers
5. **Load Balancing**: Store service can be load balanced across multiple instances

## Migration Notes

### Before (Local Memory Store)
```go
// Backfiller used local memory store
memoryStore := store.NewMemoryStore(logger, metrics)
backfiller := backfillers.NewBackfiller(memoryStore, chainClient, logger, metrics)
```

### After (gRPC Client)
```go
// Backfiller uses gRPC client to remote store service
storeClient, err := store.NewGRPCClient("localhost:8080", logger)
backfiller := backfillers.NewBackfiller(storeClient, chainClient, logger, metrics)
```

## Configuration

### Environment Variables

- `STORE_ADDR`: gRPC address of the store service (default: localhost:8080)

### Command Line Flags

- `--store-addr`: gRPC address of the store service
- `--run-once`: Run backfill once and exit
- `--source`: Backfill source (chain, glacier, off-chain)
- `--hours`: Hours to backfill from current time

## Error Handling

The gRPC client includes comprehensive error handling:

- **Connection Failures**: Graceful handling of network issues
- **Service Unavailable**: Proper error messages when store service is down
- **Timeout Handling**: Context-based timeouts for all operations
- **Retry Logic**: Built-in retry mechanisms for transient failures

## Monitoring

The gRPC client integrates with the existing logging and metrics:

- **Connection Status**: Logs connection establishment and failures
- **Request Metrics**: Tracks gRPC call latencies and success rates
- **Error Tracking**: Detailed error logging for debugging

## Example

See `examples/backfiller_grpc_example.go` for a complete example of using the backfiller with the gRPC client. 