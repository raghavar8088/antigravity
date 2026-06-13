"use client";

/**
 * Client-side Backup Manager
 * Handles periodic state backups via API calls.
 */

export class BackupManager {
  private intervalId: NodeJS.Timeout | null = null;
  private readonly backupIntervalMs = 15 * 60 * 1000; // 15 minutes

  public start() {
    if (typeof window === "undefined") return; // Only run in browser
    if (this.intervalId) return;
    this.intervalId = setInterval(() => this.runBackup(), this.backupIntervalMs);
    console.log("Backup Manager started.");
  }

  public stop() {
    if (this.intervalId) {
      clearInterval(this.intervalId);
      this.intervalId = null;
    }
  }

  private async runBackup() {
    try {
      console.log("Running periodic backup...");
      const res = await fetch("/api/storage/backup", {
        method: "POST",
        body: JSON.stringify({
          period: "daily",
          includeReports: true,
        }),
      });

      if (!res.ok) {
        throw new Error("Backup failed");
      }
    } catch (err: any) {
      console.error("Backup Manager Error:", err.message);
    }
  }
}

export const backupManager = new BackupManager();
