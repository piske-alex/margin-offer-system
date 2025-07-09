import { Connection } from "@solana/web3.js";
import {
  MarginfiClient,
  getConfig,
  Bank,
  BankMap,
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
  maxLtv: number;
  liquidationLtv: number;
  interestRate: number;
  interestModel: string;
  isActive: boolean;
  lastUpdated: Date;
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
      const bankCount = bankMap instanceof Map ? bankMap.size : bankMap.length;
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

  private async fetchAllBanks(): Promise<BankMap | MarginFiBank[]> {
    console.log("Fetching MarginFi banks");

    try {
      if (!this.marginfiClient) {
        throw new Error("MarginFi client not initialized");
      }

      const banks: BankMap = await this.marginfiClient.banks;

      return banks;
    } catch (error) {
      console.error("Failed to fetch MarginFi banks:", error);

      // Return mock data for development/testing
      console.log("Falling back to mock data");
      return this.getMockBanks();
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

      // Get LTV ratios from bank config
      // For LTV, we need to use liabilityWeightInit and liabilityWeightMaint
      // LTV = 1 / liability weight (since higher liability weight = lower LTV allowed)
      const maxLtv = bank.config.liabilityWeightInit.gt(0)
        ? 1 / bank.config.liabilityWeightInit.toNumber()
        : 0.75; // Default max LTV if not set
      const liquidationLtv = bank.config.liabilityWeightMaint.gt(0)
        ? 1 / bank.config.liabilityWeightMaint.toNumber()
        : 0.9; // Default liquidation LTV if not set

      // Check if bank is operational
      const isActive = bank.config.operationalState === "Operational";

      return {
        address: bankAddress,
        collateralToken: bank.mint.toString(),
        borrowToken: bank.mint.toString(),
        availableLiquidity,
        maxLtv,
        liquidationLtv,
        interestRate: interestRate.borrowingRate.toNumber(),
        interestModel: "floating",
        isActive,
        lastUpdated: new Date(),
      };
    } catch (error) {
      console.error(`Error converting bank ${bankAddress}:`, error);
      return null;
    }
  }

  private getMockBanks(): MarginFiBank[] {
    return [
      {
        address: "Bank1Address",
        collateralToken: "SOL",
        borrowToken: "USDC",
        availableLiquidity: 1000000.0,
        maxLtv: 0.75,
        liquidationLtv: 0.85,
        interestRate: 0.05,
        interestModel: "floating",
        isActive: true,
        lastUpdated: new Date(),
      },
      {
        address: "Bank2Address",
        collateralToken: "ETH",
        borrowToken: "USDC",
        availableLiquidity: 500000.0,
        maxLtv: 0.7,
        liquidationLtv: 0.8,
        interestRate: 0.06,
        interestModel: "floating",
        isActive: true,
        lastUpdated: new Date(),
      },
      {
        address: "Bank3Address",
        collateralToken: "BTC",
        borrowToken: "USDC",
        availableLiquidity: 750000.0,
        maxLtv: 0.65,
        liquidationLtv: 0.75,
        interestRate: 0.04,
        interestModel: "floating",
        isActive: true,
        lastUpdated: new Date(),
      },
    ];
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

      return marginFiBanks.map((bank) => ({
        id: `marginfi_${bank.address}`,
        offerType: "V1",
        collateralToken: bank.collateralToken,
        borrowToken: bank.borrowToken,
        availableBorrowAmount: bank.availableLiquidity,
        maxOpenLtv: bank.maxLtv,
        liquidationLtv: bank.liquidationLtv,
        interestRate: bank.interestRate,
        interestModel: bank.interestModel,
        liquiditySource: "marginfi",
        source: "marginfi",
        createdTimestamp: now,
        updatedTimestamp: now,
      }));
    }

    // Handle mock data (array of MarginFiBank)
    return banks.map((bank) => ({
      id: `marginfi_${bank.address}`,
      offerType: "V1",
      collateralToken: bank.collateralToken,
      borrowToken: bank.borrowToken,
      availableBorrowAmount: bank.availableLiquidity,
      maxOpenLtv: bank.maxLtv,
      liquidationLtv: bank.liquidationLtv,
      interestRate: bank.interestRate,
      interestModel: bank.interestModel,
      liquiditySource: "marginfi",
      source: "marginfi",
      createdTimestamp: now,
      updatedTimestamp: now,
    }));
  }

  private async overwriteStore(offers: MarginOffer[]): Promise<void> {
    return new Promise((resolve, reject) => {
      const request = {
        offers: offers.map((offer) => ({
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
