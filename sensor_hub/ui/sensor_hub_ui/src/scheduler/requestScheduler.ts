import { READONLY_REQUEST_CONCURRENCY } from '../environment/Environment';

/**
 * Priority tiers for scheduled read-only requests.
 * - `high`   — controllable-related work (reserved for future use); admitted before everything else.
 * - `normal` — visible read-only widgets fetching on mount.
 * - `low`    — background polling; additionally paused while a command is in flight (see preemption).
 */
export type RequestPriority = 'high' | 'normal' | 'low';

const TIERS: RequestPriority[] = ['high', 'normal', 'low'];

/** Default safety timeout after which an engaged preempt latch auto-releases, so polls can never be starved. */
const DEFAULT_PREEMPT_TIMEOUT_MS = 5000;

interface Waiter {
  start: () => void;
}

export interface RequestScheduler {
  /**
   * Run `fn` through the scheduler at the given priority. At most `maxConcurrency` scheduled tasks run at once;
   * when a slot frees the highest-priority waiter runs next (FIFO within a tier). Resolves/rejects with `fn`'s result.
   */
  schedule<T>(priority: RequestPriority, fn: () => Promise<T>): Promise<T>;
  /**
   * Run `fn` immediately (bypassing the concurrency cap — intended for a single lightweight command) while pausing
   * admission of new `low`-priority tasks. The pause is released when `fn` settles, or after `timeoutMs` as a backstop.
   */
  runWithPreemption<T>(fn: () => Promise<T>, options?: { timeoutMs?: number }): Promise<T>;
  setMaxConcurrency(n: number): void;
  getMaxConcurrency(): number;
  /** Number of scheduled tasks currently running. Excludes preemptive (command) tasks, which bypass the cap. */
  getInFlight(): number;
  getQueuedCount(priority: RequestPriority): number;
  /** True while at least one preempt latch is engaged (new `low` tasks are paused). */
  isPreempted(): boolean;
}

export function createRequestScheduler(options?: { maxConcurrency?: number }): RequestScheduler {
  let maxConcurrency = Math.max(1, options?.maxConcurrency ?? READONLY_REQUEST_CONCURRENCY);
  let inFlight = 0;
  let preemptCount = 0;
  const queues: Record<RequestPriority, Waiter[]> = { high: [], normal: [], low: [] };

  function takeNextWaiter(): Waiter | undefined {
    for (const tier of TIERS) {
      if (tier === 'low' && preemptCount > 0) continue;
      const waiter = queues[tier].shift();
      if (waiter) return waiter;
    }
    return undefined;
  }

  function pump(): void {
    while (inFlight < maxConcurrency) {
      const waiter = takeNextWaiter();
      if (!waiter) break;
      inFlight++;
      waiter.start();
    }
  }

  function schedule<T>(priority: RequestPriority, fn: () => Promise<T>): Promise<T> {
    return new Promise<T>((resolve, reject) => {
      queues[priority].push({
        start: () => {
          Promise.resolve()
            .then(fn)
            .then(resolve, reject)
            .finally(() => {
              inFlight--;
              pump();
            });
        },
      });
      pump();
    });
  }

  function runWithPreemption<T>(fn: () => Promise<T>, opts?: { timeoutMs?: number }): Promise<T> {
    preemptCount++;
    let released = false;
    const timer = setTimeout(release, opts?.timeoutMs ?? DEFAULT_PREEMPT_TIMEOUT_MS);

    function release(): void {
      if (released) return;
      released = true;
      clearTimeout(timer);
      preemptCount--;
      pump();
    }

    // Invoke fn synchronously so the command is sent immediately (bypassing the cap).
    let result: Promise<T>;
    try {
      result = Promise.resolve(fn());
    } catch (err) {
      release();
      return Promise.reject(err);
    }
    // Release the latch when the command settles, on a side branch so the returned promise's
    // resolution timing matches a bare fn() call (callers may rely on microtask ordering).
    result.then(release, release);
    return result;
  }

  return {
    schedule,
    runWithPreemption,
    setMaxConcurrency(n: number) {
      maxConcurrency = Math.max(1, n);
      pump();
    },
    getMaxConcurrency: () => maxConcurrency,
    getInFlight: () => inFlight,
    getQueuedCount: (priority: RequestPriority) => queues[priority].length,
    isPreempted: () => preemptCount > 0,
  };
}

/** App-wide singleton. All read-only widget fetches and toggle commands go through this. */
export const requestScheduler = createRequestScheduler();
