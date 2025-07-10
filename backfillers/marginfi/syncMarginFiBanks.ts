import { Connection } from "@solana/web3.js";
import {
  MarginfiClient,
  getConfig,
  Bank,
  BankMap,
  computeMaxLeverage,
} from "@mrgnlabs/marginfi-client-v2";
import { NodeWallet } from "@mrgnlabs/mrgn-common";
import * as grpc from "@grpc/grpc-js";
import * as protoLoader from "@grpc/proto-loader";
import * as dotenv from "dotenv";

dotenv.config();

// gRPC client setup
const PROTO_PATH = "../../proto/margin_offer.proto";
const packageDefinition = protoLoader.loadSync(PROTO_PATH, {
  keepCase: false, // Convert snake_case to camelCase
  longs: String,
  enums: String,
  defaults: true,
  oneofs: true,
});

const protoDescriptor = grpc.loadPackageDefinition(packageDefinition);
const marginOfferService = (protoDescriptor as any).marginoffer?.v1
  ?.MarginOfferService;

// Configuration
const STORE_GRPC_ADDRESS = process.env.STORE_GRPC_ADDRESS || "localhost:8080";
const SYNC_INTERVAL_MS = parseInt(process.env.SYNC_INTERVAL_MS || "900000"); // 15 minutes
const SOLANA_RPC_ENDPOINT =
  process.env.SOLANA_RPC_ENDPOINT || "https://api.mainnet-beta.solana.com";
const MARGINFI_PROGRAM_ID = "MFv2hKfKT5vk5wrX1Y5y3EmwQ5qkAv4DqFnpRZmUxWL";

// Types
interface MarginFiBank {
  address: string;
  collateralToken: string;
  borrowToken: string;
  availableLiquidity: number;

  interestRate: number;
  interestModel: string;
  isActive: boolean;
  lastUpdated: Date;
  group: string;
  originalBank: Bank;
}

interface MarginOffer {
  id: string;
  offerType: string;
  collateralToken: string;
  borrowToken: string;
  availableBorrowAmount: number;
  maxOpenLtv: number;
  liquidationLtv: number;
  interestRate: number;
  interestModel: string;
  liquiditySource: string;
  source?: string;
  createdTimestamp: Date;
  updatedTimestamp: Date;
}

class MarginFiSyncService {
  private client: any;
  private marginfiClient: MarginfiClient | null = null;
  private connection: Connection;
  private wallet: NodeWallet;
  private config: any;
  private isRunning: boolean = false;
  private lastSyncTime?: Date;
  private syncInterval?: NodeJS.Timeout;

  constructor() {
    // Initialize gRPC client
    this.client = new marginOfferService(
      STORE_GRPC_ADDRESS,
      grpc.credentials.createInsecure()
    );

    // Initialize Solana connection
    this.connection = new Connection(SOLANA_RPC_ENDPOINT, "confirmed");
    this.wallet = NodeWallet.local();
    this.config = getConfig();
  }

  async initialize(): Promise<void> {
    try {
      console.log("Initializing MarginFi sync service...");

      // Test gRPC connection
      await this.testGrpcConnection();

      // Initialize MarginFi client
      this.marginfiClient = await MarginfiClient.fetch(
        this.config,
        this.wallet,
        this.connection
      );

      console.log("MarginFi sync service initialized successfully");
    } catch (error) {
      console.error("Failed to initialize MarginFi sync service:", error);
      throw error;
    }
  }

  private async testGrpcConnection(): Promise<void> {
    return new Promise((resolve, reject) => {
      this.client.HealthCheck({}, (error: any, response: any) => {
        if (error) {
          reject(new Error(`gRPC connection failed: ${error.message}`));
        } else {
          console.log("gRPC connection established successfully");
          resolve();
        }
      });
    });
  }

  async start(): Promise<void> {
    if (this.isRunning) {
      console.log("MarginFi sync service is already running");
      return;
    }

    console.log(
      `Starting MarginFi sync service with ${SYNC_INTERVAL_MS}ms interval`
    );
    this.isRunning = true;

    // Run initial sync
    await this.syncBanks();

    // Start periodic sync
    this.syncInterval = setInterval(async () => {
      try {
        await this.syncBanks();
      } catch (error) {
        console.error("Periodic sync failed:", error);
      }
    }, SYNC_INTERVAL_MS);
  }

  async stop(): Promise<void> {
    if (!this.isRunning) {
      return;
    }

    console.log("Stopping MarginFi sync service...");
    this.isRunning = false;

    if (this.syncInterval) {
      clearInterval(this.syncInterval);
      this.syncInterval = undefined;
    }
  }

  async syncBanks(): Promise<void> {
    const startTime = Date.now();
    console.log("Starting MarginFi banks sync...");

    try {
      // Fetch all banks
      const bankMap = await this.fetchAllBanks();
      const bankCount = bankMap.size;
      console.log(`Fetched ${bankCount} MarginFi banks`);

      if (bankCount === 0) {
        console.warn("No banks fetched.");
      }

      // Convert banks to margin offers
      const offers = await this.convertBanksToMarginOffers(bankMap);
      console.log(`Converted ${offers.length} banks to margin offers`);

      // Overwrite the store with new data
      await this.overwriteStore(offers);

      const duration = Date.now() - startTime;
      this.lastSyncTime = new Date();

      console.log(`MarginFi banks sync completed in ${duration}ms`, {
        banks: bankCount,
        offers: offers.length,
        duration: `${duration}ms`,
      });
    } catch (error) {
      console.error("MarginFi banks sync failed:", error);
      throw error;
    }
  }

  private async fetchAllBanks(): Promise<BankMap> {
    console.log("Fetching MarginFi banks");

    try {
      if (!this.marginfiClient) {
        throw new Error("MarginFi client not initialized");
      }

      const banks: BankMap = this.marginfiClient.banks;

      return banks;
    } catch (error) {
      console.error("Failed to fetch MarginFi banks:", error);
      throw error;
    }
  }

  private async convertBankToMarginFiBank(
    bank: Bank,
    bankAddress: string
  ): Promise<MarginFiBank | null> {
    try {
      // Calculate metrics using the Bank properties
      const totalDeposits = bank.getTotalAssetQuantity().toNumber();
      const totalBorrows = bank.getTotalLiabilityQuantity().toNumber();
      const availableLiquidity = Math.max(0, totalDeposits - totalBorrows);

      const interestRate = bank.computeInterestRates();

      // Get group information from the bank
      const group = bank.group;

      // Check if bank is operational
      const isActive = bank.config.operationalState === "Operational";

      // Each bank represents a single token, so both collateral and borrow are the same
      const tokenMint = bank.mint.toString();

      return {
        address: bankAddress,
        collateralToken: tokenMint,
        borrowToken: tokenMint, // Same as collateral since each bank is single-token
        availableLiquidity,
        interestRate: interestRate.borrowingRate.toNumber(),
        interestModel: "floating",
        isActive,
        lastUpdated: new Date(),
        group: group.toString(),
        originalBank: bank,
      };
    } catch (error) {
      console.error(`Error converting bank ${bankAddress}:`, error);
      return null;
    }
  }


  private async convertBanksToMarginOffers(
    banks: BankMap | MarginFiBank[]
  ): Promise<MarginOffer[]> {
    const now = new Date();

    // Handle BankMap (real data from MarginFi)
    if (banks instanceof Map) {
      const marginFiBanks: MarginFiBank[] = [];

      for (const [bankAddress, bank] of banks.entries()) {
        const marginFiBank = await this.convertBankToMarginFiBank(
          bank,
          bankAddress
        );
        if (marginFiBank) {
          marginFiBanks.push(marginFiBank);
        }
      }
      

      // Group banks by their group (assuming all banks are in the same group for now)
      // In a real implementation, you would get the group from the bank configuration
      const bankGroups = this.groupBanksByGroup(marginFiBanks);
      
      // Create offers for all permutations within each group
      const offers: MarginOffer[] = [];
      
      for (const [groupKey, groupBanks] of bankGroups.entries()) {
        const groupOffers = this.createOffersForGroup(groupBanks, now);
        offers.push(...groupOffers);
      }

      return offers;
    }

    // Handle mock data (array of MarginFiBank)
    // For mock data, we'll create a simple group with all banks
    const bankGroups = this.groupBanksByGroup(banks);
    const offers: MarginOffer[] = [];
    
    for (const [groupKey, groupBanks] of bankGroups.entries()) {
      const groupOffers = this.createOffersForGroup(groupBanks, now);
      offers.push(...groupOffers);
    }

    return offers;
  }

  private groupBanksByGroup(banks: MarginFiBank[]): Map<string, MarginFiBank[]> {
    const groups = new Map<string, MarginFiBank[]>();
    
    // Group banks by their actual group
    for (const bank of banks) {
      const groupKey = bank.group;
      if (!groups.has(groupKey)) {
        groups.set(groupKey, []);
      }
      groups.get(groupKey)!.push(bank);
    }
    
    console.log(`Grouped ${banks.length} banks into ${groups.size} group(s)`);
    for (const [groupKey, groupBanks] of groups.entries()) {
      const tokens = [...new Set(groupBanks.map(bank => bank.collateralToken))];
      console.log(`Group "${groupKey}": ${groupBanks.length} banks with tokens: ${tokens.join(', ')}`);
    }
    
    return groups;
  }

  private createOffersForGroup(groupBanks: MarginFiBank[], now: Date): MarginOffer[] {
    const offers: MarginOffer[] = [];
    
    // Get all unique tokens in this group
    const tokens = [...new Set(groupBanks.map(bank => bank.collateralToken))];
    console.log(`Creating offers for group with tokens: ${tokens.join(', ')}`);
    
    // Create offers for all possible collateral/borrow token permutations
    for (let i = 0; i < tokens.length; i++) {
      for (let j = 0; j < tokens.length; j++) {
        if (i !== j) { // Skip self-borrowing (collateral = borrow)
          const collateralToken = tokens[i];
          const borrowToken = tokens[j];
          
          // Find the bank for the borrow token to get its properties
          const borrowBank = groupBanks.find(bank => bank.collateralToken === borrowToken);
          const depositBank = groupBanks.find(bank => bank.collateralToken === collateralToken);
          
          if (borrowBank) {
            offers.push({
              id: `marginfi_${collateralToken}_${borrowToken}`,
              offerType: "V1",
              collateralToken,
              borrowToken,
              availableBorrowAmount: borrowBank.availableLiquidity,
              maxOpenLtv: computeMaxLeverage(depositBank.originalBank, borrowBank.originalBank).ltv-0.01,
              liquidationLtv: computeMaxLeverage(depositBank.originalBank, borrowBank.originalBank).ltv,
              interestRate: borrowBank.interestRate,
              interestModel: borrowBank.interestModel,
              liquiditySource: "marginfi",
              source: "marginfi",
              createdTimestamp: now,
              updatedTimestamp: now,
            });
          }
        }
      }
    }
    
    console.log(`Created ${offers.length} offers for group (${tokens.length} tokens = ${tokens.length * (tokens.length - 1)} permutations)`);
    return offers;
  }

  private async overwriteStore(offers: MarginOffer[]): Promise<void> {
    return new Promise((resolve, reject) => {
      const request = {
        offers: offers.filter((offer) => offer.maxOpenLtv > 0).map((offer) => ({
          id: offer.id,
          offerType: offer.offerType,
          collateralToken: offer.collateralToken,
          borrowToken: offer.borrowToken,
          availableBorrowAmount: offer.availableBorrowAmount,
          maxOpenLtv: offer.maxOpenLtv,
          liquidationLtv: offer.liquidationLtv,
          interestRate: offer.interestRate,
          interestModel: offer.interestModel,
          liquiditySource: offer.liquiditySource,
          source: offer.source,
          createdTimestamp: {
            seconds: Math.floor(offer.createdTimestamp.getTime() / 1000),
            nanos: (offer.createdTimestamp.getTime() % 1000) * 1000000,
          },
          updatedTimestamp: {
            seconds: Math.floor(offer.updatedTimestamp.getTime() / 1000),
            nanos: (offer.updatedTimestamp.getTime() % 1000) * 1000000,
          },
        })),
        // Add filter to only overwrite MarginFi offers
        filter: {
          source: "marginfi",
        },
        validateAll: true,
        dryRun: false,
      };

      this.client.BulkOverwriteMarginOffers(
        request,
        (error: any, response: any) => {
          if (error) {
            console.error("Failed to overwrite store:", error);
            reject(error);
          } else {
            console.log("Store overwritten successfully:", {
              deletedCount: response.deletedCount,
              createdCount: response.createdCount,
              deletedIds: response.deletedIds || [],
              createdIds: response.createdIds || [],
            });
            resolve();
          }
        }
      );
    });
  }

  async forceSync(): Promise<void> {
    console.log("Force sync triggered");
    await this.syncBanks();
  }

  getStatus(): any {
    return {
      isRunning: this.isRunning,
      lastSyncTime: this.lastSyncTime,
      nextSyncTime: this.lastSyncTime
        ? new Date(this.lastSyncTime.getTime() + SYNC_INTERVAL_MS)
        : undefined,
      syncIntervalMs: SYNC_INTERVAL_MS,
      programId: MARGINFI_PROGRAM_ID,
      grpcAddress: STORE_GRPC_ADDRESS,
    };
  }
}

// Main execution
async function main() {
  const syncService = new MarginFiSyncService();

  try {
    await syncService.initialize();
    await syncService.start();

    console.log("MarginFi sync service started successfully");
    console.log("Status:", syncService.getStatus());

    // Keep the process running
    process.on("SIGINT", async () => {
      console.log("Received SIGINT, shutting down...");
      await syncService.stop();
      process.exit(0);
    });

    process.on("SIGTERM", async () => {
      console.log("Received SIGTERM, shutting down...");
      await syncService.stop();
      process.exit(0);
    });
  } catch (error) {
    console.error("Failed to start MarginFi sync service:", error);
    process.exit(1);
  }
}

// Export for use as module
export { MarginFiSyncService };

// Run if this file is executed directly
if (require.main === module) {
  main().catch(console.error);
}
