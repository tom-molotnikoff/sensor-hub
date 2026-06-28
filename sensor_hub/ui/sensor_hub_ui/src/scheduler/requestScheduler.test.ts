import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createRequestScheduler, type RequestScheduler } from './requestScheduler';

/** A promise whose resolution we control, plus a record of whether `fn` has started running. */
function deferred<T = void>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

/** Flush pending microtasks so admitted `start()` callbacks invoke their `fn`. */
const flush = () => Promise.resolve().then(() => Promise.resolve());

describe('requestScheduler', () => {
  let scheduler: RequestScheduler;

  beforeEach(() => {
    scheduler = createRequestScheduler({ maxConcurrency: 2 });
  });

  it('never runs more than maxConcurrency tasks at once', async () => {
    const gates = Array.from({ length: 5 }, () => deferred());
    let running = 0;
    let peak = 0;

    const results = gates.map((gate) =>
      scheduler.schedule('normal', async () => {
        running++;
        peak = Math.max(peak, running);
        await gate.promise;
        running--;
      }),
    );

    await flush();
    expect(scheduler.getInFlight()).toBe(2);
    expect(running).toBe(2);

    // Release one slot; exactly one more should be admitted.
    gates[0].resolve();
    await flush();
    expect(scheduler.getInFlight()).toBe(2);

    gates.forEach((g) => g.resolve());
    await Promise.all(results);
    expect(peak).toBe(2);
    expect(scheduler.getInFlight()).toBe(0);
  });

  it('admits higher priority waiters before lower, FIFO within a tier', async () => {
    const order: string[] = [];
    const block = deferred();

    // Occupy both slots so everything else queues.
    scheduler.schedule('normal', () => block.promise);
    scheduler.schedule('normal', () => block.promise);
    await flush();
    expect(scheduler.getInFlight()).toBe(2);

    // Enqueue a mix while saturated.
    const mk = (label: string) => () => {
      order.push(label);
      return Promise.resolve();
    };
    scheduler.schedule('low', mk('low-1'));
    scheduler.schedule('normal', mk('normal-1'));
    scheduler.schedule('high', mk('high-1'));
    scheduler.schedule('normal', mk('normal-2'));
    scheduler.schedule('high', mk('high-2'));

    // Drain: release the two blockers, then let the queue flow.
    block.resolve();
    for (let i = 0; i < 10; i++) await flush();

    expect(order).toEqual(['high-1', 'high-2', 'normal-1', 'normal-2', 'low-1']);
  });

  it('propagates resolution and rejection of the wrapped fn', async () => {
    await expect(scheduler.schedule('normal', () => Promise.resolve(42))).resolves.toBe(42);
    await expect(scheduler.schedule('normal', () => Promise.reject(new Error('boom')))).rejects.toThrow('boom');
  });

  it('admits more work immediately when maxConcurrency is raised', async () => {
    const block = deferred();
    for (let i = 0; i < 4; i++) scheduler.schedule('normal', () => block.promise);
    await flush();
    expect(scheduler.getInFlight()).toBe(2);

    scheduler.setMaxConcurrency(4);
    await flush();
    expect(scheduler.getInFlight()).toBe(4);

    block.resolve();
  });

  describe('preemption', () => {
    it('pauses new low-priority admits while a command is in flight, then resumes', async () => {
      const idle = createRequestScheduler({ maxConcurrency: 2 });
      const command = deferred();
      let lowRan = false;

      const cmd = idle.runWithPreemption(() => command.promise);
      expect(idle.isPreempted()).toBe(true);

      idle.schedule('low', () => {
        lowRan = true;
        return Promise.resolve();
      });
      await flush();
      // Slots are free, but the low task is held back by the engaged latch.
      expect(lowRan).toBe(false);
      expect(idle.getQueuedCount('low')).toBe(1);

      // normal tasks are NOT affected by preemption.
      let normalRan = false;
      idle.schedule('normal', () => {
        normalRan = true;
        return Promise.resolve();
      });
      await flush();
      expect(normalRan).toBe(true);

      command.resolve();
      await cmd;
      await flush();
      expect(idle.isPreempted()).toBe(false);
      expect(lowRan).toBe(true);
    });

    it('auto-releases the latch after the timeout so polls are never starved', async () => {
      vi.useFakeTimers();
      try {
        const s = createRequestScheduler({ maxConcurrency: 2 });
        const neverResolves = deferred();
        s.runWithPreemption(() => neverResolves.promise, { timeoutMs: 1000 });
        expect(s.isPreempted()).toBe(true);

        let lowRan = false;
        s.schedule('low', () => {
          lowRan = true;
          return Promise.resolve();
        });
        await Promise.resolve();
        expect(lowRan).toBe(false);

        vi.advanceTimersByTime(1000);
        expect(s.isPreempted()).toBe(false);
        await Promise.resolve().then(() => Promise.resolve());
        expect(lowRan).toBe(true);
      } finally {
        vi.useRealTimers();
      }
    });

    it('runs the preemptive task immediately even when the cap is saturated', async () => {
      const block = deferred();
      scheduler.schedule('normal', () => block.promise);
      scheduler.schedule('normal', () => block.promise);
      await flush();
      expect(scheduler.getInFlight()).toBe(2);

      let commandRan = false;
      const cmd = scheduler.runWithPreemption(() => {
        commandRan = true;
        return Promise.resolve('ok');
      });
      await flush();
      expect(commandRan).toBe(true);
      await expect(cmd).resolves.toBe('ok');

      block.resolve();
    });
  });
});

afterEach(() => {
  vi.useRealTimers();
});
