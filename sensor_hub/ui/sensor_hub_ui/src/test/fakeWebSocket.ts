/**
 * A WebSocket test double installed on globalThis, so hooks that open real
 * sockets (useProperties, useSensors, ...) can be exercised unmocked. Tests
 * drive it from the server side with `serverSends`.
 */
export class FakeWebSocket {
  static instances: FakeWebSocket[] = [];

  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSING = 2;
  static readonly CLOSED = 3;

  url: string;
  readyState = FakeWebSocket.OPEN;
  onopen: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  onclose: ((event: CloseEvent) => void) | null = null;

  constructor(url: string) {
    this.url = url;
    FakeWebSocket.instances.push(this);
  }

  send() {}

  close() {
    this.readyState = FakeWebSocket.CLOSED;
  }

  /** Deliver a message as if the server sent it. Wrap the call in act(). */
  serverSends(data: string) {
    this.onmessage?.(new MessageEvent('message', { data }));
  }

  serverCloses() {
    this.readyState = FakeWebSocket.CLOSED;
    this.onclose?.(new CloseEvent('close'));
  }
}

/** Replace globalThis.WebSocket with the fake. Returns a restore function. */
export function installFakeWebSocket(): () => void {
  const original = globalThis.WebSocket;
  FakeWebSocket.instances = [];
  (globalThis as { WebSocket: unknown }).WebSocket = FakeWebSocket;
  return () => {
    (globalThis as { WebSocket: unknown }).WebSocket = original;
  };
}
