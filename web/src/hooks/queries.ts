// TanStack Query hooks. Live data refetches on an interval (when live mode is on)
// and whenever the window regains focus.
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { metaApi, ticketsApi } from '../api/endpoints';
import type { TicketFilters } from '../api/types';
import { useLiveRefresh } from './useLiveRefresh';

export const queryKeys = {
  tickets: (filters?: TicketFilters) => (filters ? ['tickets', filters] : ['tickets']),
  ticket: (id: string) => ['ticket', id],
  timeline: (id: string) => ['timeline', id],
  summary: ['summary'],
  enums: ['enums'],
  agents: ['agents'],
};

export function useTickets(filters: TicketFilters, enabled = true) {
  const { interval } = useLiveRefresh();
  return useQuery({
    queryKey: queryKeys.tickets(filters),
    queryFn: () => ticketsApi.list(filters),
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

export function useSummary() {
  const { interval } = useLiveRefresh();
  return useQuery({ queryKey: queryKeys.summary, queryFn: metaApi.summary, refetchInterval: interval });
}

export function useEnums() {
  return useQuery({ queryKey: queryKeys.enums, queryFn: metaApi.enums, staleTime: Infinity });
}

export function useAgents(enabled: boolean) {
  return useQuery({ queryKey: queryKeys.agents, queryFn: metaApi.agents, enabled });
}
