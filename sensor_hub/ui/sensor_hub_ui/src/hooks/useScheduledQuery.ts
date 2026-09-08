import { useCallback, useEffect, useRef, useState } from 'react';
import { subscribeToPollClock } from '../dashboard/pollClock';
import { useWidgetStateReport, useWidgetViewport, type WidgetState } from '../dashboard/WidgetContext';
import { requestScheduler, type RequestPriority } from '../scheduler/requestScheduler';
import { logger } from '../tools/logger';

export type ScheduledQueryStatus = 'held' | 'queued' | 'loading' | 'fresh' | 'error';

export interface ScheduledQueryOptions {
    pollIntervalMs?: number;
    enabled?: boolean;
    deps: unknown[];
}

export interface ScheduledQueryResult<T> {
    data: T | undefined;
    status: ScheduledQueryStatus;
    error: unknown;
    isLoading: boolean;
    refetch: () => Promise<void>;
}

interface Snapshot<T> {
    status: ScheduledQueryStatus;
    data: T | undefined;
    error: unknown;
}

interface Run {
    id: number;
    controller: AbortController;
    started: boolean;
    settled: Promise<void>;
}

function held<T>(): Snapshot<T> {
    return { status: 'held', data: undefined, error: null };
}

function abortReason(message: string): unknown {
    return new DOMException(message, 'AbortError');
}

function sameDeps(a: unknown[], b: unknown[]): boolean {
    return a.length === b.length && a.every((value, index) => Object.is(value, b[index]));
}

function widgetStateOf(snapshot: Snapshot<unknown>): WidgetState {
    if (snapshot.status === 'error') return 'error';
    if (snapshot.data !== undefined) return 'populated';
    if (snapshot.status === 'held') return 'held';
    return 'loading';
}

export function useScheduledQuery<T>(
    fetcher: (signal: AbortSignal) => Promise<T>,
    { pollIntervalMs, enabled = true, deps }: ScheduledQueryOptions,
): ScheduledQueryResult<T> {
    const { visible, kind } = useWidgetViewport();

    const [queryKey, setQueryKey] = useState(deps);
    const [committed, setSnapshot] = useState<Snapshot<T>>(held);
    const supersededByDeps = !sameDeps(queryKey, deps);
    if (supersededByDeps) {
        setQueryKey(deps);
        setSnapshot(held);
    }
    const snapshot = supersededByDeps ? held<T>() : committed;

    const fetcherRef = useRef(fetcher);
    useEffect(() => {
        fetcherRef.current = fetcher;
    });

    const runRef = useRef<Run | null>(null);
    const runIdRef = useRef(0);
    const lastAttemptAtRef = useRef<number | null>(null);
    const lastQueryKeyRef = useRef(queryKey);
    const refetchRef = useRef<() => Promise<void>>(() => Promise.resolve());
    const refetch = useCallback(() => refetchRef.current(), []);

    useEffect(() => {
        if (lastQueryKeyRef.current !== queryKey) {
            lastQueryKeyRef.current = queryKey;
            lastAttemptAtRef.current = null;
            discardRun('Query inputs changed');
        }

        refetchRef.current = () => {
            if (!enabled) return Promise.resolve();
            if (runRef.current) return runRef.current.settled;
            lastAttemptAtRef.current = null;
            return startFetch(initialPriority());
        };

        if (!enabled) {
            discardRun('Query disabled');
            return;
        }

        if (!visible) {
            if (runRef.current && !runRef.current.started) abortQueuedRun();
            return;
        }

        maybeFetch();
        if (pollIntervalMs == null) return;
        return subscribeToPollClock(maybeFetch);

        function discardRun(message: string): void {
            const run = runRef.current;
            if (!run) return;
            runIdRef.current++;
            runRef.current = null;
            run.controller.abort(abortReason(message));
        }

        function abortQueuedRun(): void {
            runRef.current?.controller.abort(abortReason('Widget left the viewport'));
        }

        function initialPriority(): RequestPriority {
            return kind === 'controllable' ? 'high' : 'normal';
        }

        function maybeFetch(): void {
            if (runRef.current) return;
            const lastAttemptAt = lastAttemptAtRef.current;
            if (lastAttemptAt === null) {
                startFetch(initialPriority());
                return;
            }
            if (pollIntervalMs != null && Date.now() - lastAttemptAt >= pollIntervalMs) startFetch('low');
        }

        function startFetch(priority: RequestPriority): Promise<void> {
            const id = ++runIdRef.current;
            const run: Run = { id, controller: new AbortController(), started: false, settled: Promise.resolve() };
            runRef.current = run;
            setSnapshot((prev) => ({ ...prev, status: 'queued' }));

            run.settled = requestScheduler
                .schedule(priority, () => {
                    run.started = true;
                    setSnapshot((prev) => ({ ...prev, status: 'loading' }));
                    return fetcherRef.current(run.controller.signal);
                }, { signal: run.controller.signal })
                .then((data) => {
                    if (runIdRef.current !== id) return;
                    lastAttemptAtRef.current = Date.now();
                    setSnapshot({ status: 'fresh', data, error: null });
                })
                .catch((err: unknown) => {
                    if (runIdRef.current !== id) return;
                    if (run.controller.signal.aborted) {
                        setSnapshot((prev) => ({ ...prev, status: prev.data === undefined ? 'held' : 'fresh' }));
                        return;
                    }
                    lastAttemptAtRef.current = Date.now();
                    logger.error('Scheduled query failed', err);
                    setSnapshot((prev) => ({ ...prev, status: 'error', error: err }));
                })
                .finally(() => {
                    if (runRef.current?.id === id) runRef.current = null;
                });

            return run.settled;
        }
    }, [queryKey, visible, kind, pollIntervalMs, enabled]);

    useEffect(() => () => {
        const run = runRef.current;
        runIdRef.current++;
        runRef.current = null;
        if (run && !run.started) run.controller.abort(abortReason('Widget unmounted'));
    }, []);

    useWidgetStateReport(enabled ? widgetStateOf(snapshot) : 'populated');

    return {
        data: snapshot.data,
        status: snapshot.status,
        error: snapshot.error,
        isLoading: enabled && snapshot.data === undefined && snapshot.status !== 'error',
        refetch,
    };
}
