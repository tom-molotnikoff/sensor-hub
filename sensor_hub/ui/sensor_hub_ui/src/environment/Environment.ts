export const API_BASE = import.meta.env.VITE_API_BASE || '/api';
export const WEBSOCKET_BASE = import.meta.env.VITE_WEBSOCKET_BASE || '/api';

/**
 * Maximum number of read-only REST requests the client will have in flight to the backend at once.
 * Caps the dashboard's on-load request burst so controllable widgets (and their commands) aren't
 * starved by heavy chart queries on slow deployments. Tunable per deployment via VITE_READONLY_REQUEST_CONCURRENCY.
 */
export const READONLY_REQUEST_CONCURRENCY = Number(import.meta.env.VITE_READONLY_REQUEST_CONCURRENCY) || 4;

