import { createContext, ReactNode, useContext, useEffect, useState } from 'react';

/** Auto-refresh choices (0 = off), like Grafana. */
export const REFRESH_OPTIONS = [
  { ms: 0, label: 'Off' },
  { ms: 5_000, label: '5s' },
  { ms: 10_000, label: '10s' },
  { ms: 30_000, label: '30s' },
  { ms: 60_000, label: '1m' },
  { ms: 300_000, label: '5m' },
  { ms: 900_000, label: '15m' },
  { ms: 1_800_000, label: '30m' },
  { ms: 3_600_000, label: '1h' },
];
export const DEFAULT_REFRESH_MS = 10_000;
const STORAGE_KEY = 'refreshIntervalMs';

function readStored(): number {
  const stored = Number(localStorage.getItem(STORAGE_KEY));
  if (localStorage.getItem(STORAGE_KEY) !== null && REFRESH_OPTIONS.some((o) => o.ms === stored)) return stored;
  return localStorage.getItem('liveRefresh') === 'off' ? 0 : DEFAULT_REFRESH_MS; // migrate the old on/off toggle
}

interface LiveRefreshValue {
  intervalMs: number;
  setIntervalMs: (ms: number) => void;
  live: boolean;
  /** Value for TanStack Query's refetchInterval. */
  interval: number | false;
}

const LiveRefreshContext = createContext<LiveRefreshValue>({
  intervalMs: DEFAULT_REFRESH_MS,
  setIntervalMs: () => {},
  live: true,
  interval: DEFAULT_REFRESH_MS,
});

/** Auto-refresh is on (10s) by default; the user's choice is remembered. */
export function LiveRefreshProvider({ children }: { children: ReactNode }) {
  const [intervalMs, setIntervalMs] = useState(readStored);

  useEffect(() => {
    localStorage.setItem(STORAGE_KEY, String(intervalMs));
  }, [intervalMs]);

  const live = intervalMs > 0;
  return (
    <LiveRefreshContext.Provider value={{ intervalMs, setIntervalMs, live, interval: live ? intervalMs : false }}>
      {children}
    </LiveRefreshContext.Provider>
  );
}

export const useLiveRefresh = () => useContext(LiveRefreshContext);
