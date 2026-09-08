const TICK_MS = 1000;

type Tick = () => void;

const subscribers = new Set<Tick>();
let timer: number | null = null;

function fireAll(): void {
    for (const tick of Array.from(subscribers)) tick();
}

function shouldRun(): boolean {
    return subscribers.size > 0 && document.visibilityState !== 'hidden';
}

function syncTimer(): void {
    if (shouldRun()) {
        if (timer === null) timer = window.setInterval(fireAll, TICK_MS);
        return;
    }
    if (timer !== null) {
        window.clearInterval(timer);
        timer = null;
    }
}

function onVisibilityChange(): void {
    syncTimer();
    if (document.visibilityState !== 'hidden') fireAll();
}

export function subscribeToPollClock(onTick: Tick): () => void {
    if (subscribers.size === 0) document.addEventListener('visibilitychange', onVisibilityChange);
    subscribers.add(onTick);
    syncTimer();

    return () => {
        subscribers.delete(onTick);
        if (subscribers.size === 0) document.removeEventListener('visibilitychange', onVisibilityChange);
        syncTimer();
    };
}
