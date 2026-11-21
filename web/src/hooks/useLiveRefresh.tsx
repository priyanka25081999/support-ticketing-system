import { createContext, ReactNode, useContext, useEffect, useState } from 'react';

/** How often live data is refetched. */
export const REFRESH_INTERVAL_MS = 10_000;
const STORAGE_KEY = 'liveRefresh';

interface LiveRefreshValue {
  live: boolean;
  setLive: (live: boolean) => void;
  /** Value for TanStack Query's refetchInterval. */
  interval: number | false;
}

const LiveRefreshContext = createContext<LiveRefreshValue>({ live: true, setLive: () => {}, interval: REFRESH_INTERVAL_MS });

/** Live auto-refresh is ON by default; the user's choice is remembered. */
export function LiveRefreshProvider({ children }: { children: ReactNode }) {
  const [live, setLive] = useState(() => localStorage.getItem(STORAGE_KEY) !== 'off');

  useEffect(() => {
    localStorage.setItem(STORAGE_KEY, live ? 'on' : 'off');
  }, [live]);

  return (
    <LiveRefreshContext.Provider value={{ live, setLive, interval: live ? REFRESH_INTERVAL_MS : false }}>
      {children}
    </LiveRefreshContext.Provider>
  );
}

export const useLiveRefresh = () => useContext(LiveRefreshContext);
