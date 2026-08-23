import { useEffect, useRef } from 'react';
import { logger } from '../tools/logger';

interface UseReconnectingWebSocketOptions {
    url: string;
    /** Called on each message received. */
    onMessage: (event: MessageEvent) => void;
    /** Whether the connection should be active. */
    enabled?: boolean;
    /** Maximum reconnect delay in ms (default 30000). */
    maxDelay?: number;
}

const BASE_DELAY = 1000;

/**
 * Opens a WebSocket connection and automatically reconnects with
 * exponential backoff when the connection is lost.
 */
export function useReconnectingWebSocket({
    url,
    onMessage,
    enabled = true,
    maxDelay = 30_000,
}: UseReconnectingWebSocketOptions) {
    const wsRef = useRef<WebSocket | null>(null);
    const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
    const delayRef = useRef(BASE_DELAY);
    const mountedRef = useRef(true);

    // Keep callbacks in refs to avoid effect dependency churn
    const onMessageRef = useRef(onMessage);
    useEffect(() => {
        onMessageRef.current = onMessage;
    });

    useEffect(() => {
        mountedRef.current = true;
        if (!enabled) return;

        delayRef.current = BASE_DELAY;

        function connect() {
            if (!mountedRef.current) return;

            const ws = new WebSocket(url);
            wsRef.current = ws;

            ws.onmessage = (event) => {
                // Reset backoff on successful data
                delayRef.current = BASE_DELAY;
                onMessageRef.current(event);
            };

            ws.onerror = (err) => {
                logger.error(`WebSocket error (${url}):`, err);
            };

            ws.onclose = () => {
                if (!mountedRef.current) return;
                logger.debug(`WebSocket closed (${url}), reconnecting in ${delayRef.current}ms`);
                reconnectTimerRef.current = setTimeout(() => {
                    delayRef.current = Math.min(delayRef.current * 2, maxDelay);
                    connect();
                }, delayRef.current);
            };
        }

        connect();

        return () => {
            mountedRef.current = false;
            clearTimeout(reconnectTimerRef.current);
            wsRef.current?.close();
        };
    }, [url, maxDelay, enabled]);
}
