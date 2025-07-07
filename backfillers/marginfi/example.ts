import { MarginFiSyncService } from './syncMarginFiBanks';

async function runExample() {
  console.log('Starting MarginFi sync service example...');
  
  const syncService = new MarginFiSyncService();
  
  try {
    // Initialize the service
    console.log('Initializing...');
    await syncService.initialize();
    
    // Get initial status
    console.log('Initial status:', syncService.getStatus());
    
    // Run a one-time sync
    console.log('Running one-time sync...');
    await syncService.forceSync();
    
    // Start periodic syncing
    console.log('Starting periodic sync...');
    await syncService.start();
    
    // Let it run for a bit
    console.log('Service is running. Press Ctrl+C to stop.');
    
    // Keep the process alive
    process.on('SIGINT', async () => {
      console.log('\nShutting down...');
      await syncService.stop();
      process.exit(0);
    });
    
  } catch (error) {
    console.error('Example failed:', error);
    process.exit(1);
  }
}

// Run the example
runExample().catch(console.error); 