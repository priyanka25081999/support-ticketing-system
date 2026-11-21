// TanStack Query hooks. Live data refetches on an interval (when live mode is on)
// and whenever the window regains focus.
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { metaApi, ticketsApi } from '../api/endpoints';
import type { TicketFilters } from '../api/types';
import { TimeRange, browserTimeZone, rangeKey, resolveRange } from '../utils/timeRange';
import { useLiveRefresh } from './useLiveRefresh';

export const queryKeys = {
  tickets: (filters?: TicketFilters) => (filters ? ['tickets', filters] : ['tickets']),
  ticket: (id: string) => ['ticket', id],
  timeline: (id: string) => ['timeline', id],
  summary: ['summary'],
  enums: ['enums'],
  agents: ['agents'],
};

/** range (optional) filters by created date; relative ranges are resolved on every fetch. */
export function useTickets(filters: TicketFilters, range?: TimeRange, enabled = true) {
  const { interval } = useLiveRefresh();
  return useQuery({
    queryKey: [...queryKeys.tickets(filters), range ? rangeKey(range) : 'all'],
    queryFn: () => {
      const { from, to } = range ? resolveRange(range) : {};
      return ticketsApi.list({ ...filters, createdFrom: from, createdTo: to });
    },
    refetchInterval: interval,
    placeholderData: keepPreviousData, // keep the table visible while changing filters/pages
    enabled,
  });
}

export function useTicket(id: string) {
  const { interval } = useLiveRefresh();
  return useQuery({ queryKey: queryKeys.ticket(id), queryFn: () => ticketsApi.get(id), refetchInterval: interval });
}

export function useTimeline(id: string) {
  const { interval } = useLiveRefresh();
  return useQuery({ queryKey: queryKeys.timeline(id), queryFn: () => ticketsApi.timeline(id), refetchInterval: interval });
}

export function useSummary(range: TimeRange) {
  const { interval } = useLiveRefresh();
  return useQuery({
    queryKey: [...queryKeys.summary, rangeKey(range)],
    queryFn: () => metaApi.summary({ ...resolveRange(range), tz: browserTimeZone() }),
    refetchInterval: interval,
    placeholderData: keepPreviousData,
  });
}

export function useEnums() {
  return useQuery({ queryKey: queryKeys.enums, queryFn: metaApi.enums, staleTime: Infinity });
}

export function useAgents(enabled: boolean) {
  return useQuery({ queryKey: queryKeys.agents, queryFn: metaApi.agents, enabled });
}
