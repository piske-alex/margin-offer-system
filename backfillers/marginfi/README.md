# MarginFi Sync Service

A TypeScript service that periodically queries all MarginFi banks and maps them to margin offers, then overwrites the store via gRPC.

## Features

- **Periodic Sync**: Automatically syncs MarginFi banks every 15 minutes (configurable)
- **gRPC Integration**: Communicates with the margin offer store via gRPC
- **Bank Parsing**: Fetches and parses MarginFi bank accounts from Solana
- **Data Mapping**: Converts MarginFi banks to standardized margin offers
- **Store Overwrite**: Completely replaces store data with fresh bank information
- **Error Handling**: Robust error handling with fallback to mock data
- **Monitoring**: Comprehensive logging and status reporting

## Prerequisites

- Node.js 18+ and npm
- Access to Solana RPC endpoint
- Running margin offer store gRPC service

## Installation

```bash
cd backfillers/marginfi
npm install
```

## Configuration

### Environment Variables

```bash
# gRPC store service address
export STORE_GRPC_ADDRESS=localhost:8080

# Sync interval in milliseconds (default: 15 minutes)
export SYNC_INTERVAL_MS=900000

# Solana RPC endpoint (optional, defaults to mainnet-beta)
export SOLANA_RPC_ENDPOINT=https://api.mainnet-beta.solana.com
```

### MarginFi Program ID

The service uses the official MarginFi v2 program ID:
```
MFv2hKfKT5vk5wrX1Y5y3EmwQ5qkAv4DqFnpRZmUxWL
```

## Usage

### Start the Service

```bash
# Development mode with auto-restart
npm run dev

# Production mode
npm start

# Build and run
npm run build
node dist/syncMarginFiBanks.js
```

### Programmatic Usage

```typescript
import { MarginFiSyncService } from './syncMarginFiBanks';

const syncService = new MarginFiSyncService();

// Initialize and start
await syncService.initialize();
await syncService.start();

// Get status
console.log(syncService.getStatus());

// Force immediate sync
await syncService.forceSync();

// Stop the service
await syncService.stop();
```

## How It Works

### 1. Initialization
- Establishes gRPC connection to the store service
- Initializes Solana connection and MarginFi client
- Tests connectivity to both services

### 2. Bank Fetching
- Queries all bank accounts from the MarginFi program using `getProgramAccounts`
- Filters accounts by data size (approximate bank account size)
- Parses account data to extract bank information

### 3. Data Mapping
- Converts MarginFi banks to standardized margin offers
- Maps fields:
  - `address` → `id` (with `marginfi_` prefix)
  - `collateralToken` → `collateralToken`
  - `borrowToken` → `borrowToken`
  - `availableLiquidity` → `availableBorrowAmount`
  - `maxLtv` → `maxOpenLTV`
  - `liquidationLtv` → `liquidationLTV`
  - `interestRate` → `interestRate`
  - `interestModel` → `interestModel`
  - Sets `liquiditySource` to `"marginfi"`

### 4. Store Overwrite
- Uses `BulkOverwriteMarginOffers` gRPC call
- Completely replaces all margin offers with fresh bank data
- Provides detailed response with deleted/created counts

### 5. Periodic Execution
- Runs initial sync on startup
- Continues syncing at configured intervals
- Handles errors gracefully with logging

## Data Flow

```
Solana RPC → MarginFi Program → Bank Accounts → Parse → Margin Offers → gRPC Store → Overwrite
```

## Error Handling

- **gRPC Connection Failures**: Logs error and continues with mock data
- **Solana RPC Failures**: Falls back to mock data for development
- **Bank Parsing Errors**: Skips individual banks, continues with others
- **Store Overwrite Failures**: Logs error and retries on next cycle

## Monitoring

### Status Information

```typescript
const status = syncService.getStatus();
// Returns:
{
  isRunning: boolean,
  lastSyncTime: Date,
  nextSyncTime: Date,
  syncIntervalMs: number,
  programId: string,
  grpcAddress: string
}
```

### Logging

The service provides comprehensive logging:
- Initialization status
- Sync start/completion with timing
- Bank count and offer count
- Error details with context
- gRPC operation results

## Development

### Mock Data

During development, if bank fetching fails, the service returns mock data:
- SOL/USDC bank with 75% LTV
- ETH/USDC bank with 70% LTV  
- BTC/USDC bank with 65% LTV

### Testing

```bash
# Run tests
npm test

# Run with specific environment
STORE_GRPC_ADDRESS=localhost:8080 npm start
```

## Integration with Main System

This service integrates with the main margin offer system:

1. **Store Service**: Communicates via gRPC on port 8080
2. **Backfiller**: Can be run alongside the Go backfiller
3. **ETL Pipeline**: Provides real-time MarginFi data
4. **API**: MarginFi offers available through the main API

## Troubleshooting

### Common Issues

1. **gRPC Connection Failed**
   - Ensure store service is running on the correct port
   - Check firewall settings
   - Verify protobuf file path

2. **Solana RPC Errors**
   - Check RPC endpoint availability
   - Verify program ID is correct
   - Check rate limits

3. **Bank Parsing Failures**
   - Verify account data size filter
   - Check MarginFi client version compatibility
   - Review bank account structure changes

### Debug Mode

Enable detailed logging by setting:
```bash
export DEBUG=true
```

## License

MIT License - see LICENSE file for details. 