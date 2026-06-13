/**
 * Async non-blocking write queue for dual persistence.
 * All filesystem writes are enqueued here so they never block the API response path.
 * Retries up to maxRetries times with exponential backoff before giving up.
 */

export interface WriteQueueStats {
  queued: number;
  succeeded: number;
  failed: number;
  retried: number;
  queueDepth: number;
}

interface WriteTask {
  id: string;
  label: string;
  fn: () => Promise<void>;
  retries: number;
  maxRetries: number;
  createdAt: number;
}

class WriteQueue {
  private readonly tasks: WriteTask[] = [];
  private processing = false;
  private readonly stats: Omit<WriteQueueStats, "queueDepth"> = {
    queued: 0,
    succeeded: 0,
    failed: 0,
    retried: 0,
  };

  enqueue(label: string, fn: () => Promise<void>, maxRetries = 3): void {
    this.tasks.push({
      id: `wq-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
      label,
      fn,
      retries: 0,
      maxRetries,
      createdAt: Date.now(),
    });
    this.stats.queued++;
    if (!this.processing) void this.process();
  }

  private async process(): Promise<void> {
    this.processing = true;
    while (this.tasks.length > 0) {
      const task = this.tasks.shift()!;
      try {
        await task.fn();
        this.stats.succeeded++;
      } catch (err) {
        if (task.retries < task.maxRetries) {
          task.retries++;
          this.stats.retried++;
          await sleep(100 * 2 ** (task.retries - 1)); // 100 → 200 → 400 ms
          this.tasks.unshift(task);
        } else {
          this.stats.failed++;
          console.error(
            `[WriteQueue] ${task.label} failed after ${task.maxRetries} retries:`,
            err instanceof Error ? err.message : err,
          );
        }
      }
    }
    this.processing = false;
  }

  getStats(): WriteQueueStats {
    return { ...this.stats, queueDepth: this.tasks.length };
  }

  /** Wait until the queue drains (useful in tests). */
  async drain(): Promise<void> {
    while (this.processing || this.tasks.length > 0) {
      await sleep(10);
    }
  }
}

function sleep(ms: number): Promise<void> {
  return new Promise((r) => setTimeout(r, ms));
}

export const writeQueue = new WriteQueue();
