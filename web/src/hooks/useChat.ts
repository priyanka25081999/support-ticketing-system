import { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { openChatStream } from '../api/client';
import { chatApi } from '../api/endpoints';
import type { ChatMessage, ChatThread } from '../api/types';
import { parseSSE } from '../utils/sse';

export type ChatStatus = 'connecting' | 'live' | 'reconnecting';
const MAX_RETRY_DELAY_MS = 10_000;

/**
 * Chat history (REST) + live updates (Server-Sent Events, HTTP/2 when served over TLS).
 * Reconnects with exponential backoff and reloads history after a reconnect
 * so no message is missed.
 */
export function useChat(ticketId: string) {
  const queryClient = useQueryClient();
  const key = ['chat', ticketId];
  const thread = useQuery({ queryKey: key, queryFn: () => chatApi.thread(ticketId) });
  const [status, setStatus] = useState<ChatStatus>('connecting');

  useEffect(() => {
    const chatKey = ['chat', ticketId];
    const append = (msg: ChatMessage) =>
      queryClient.setQueryData<ChatThread>(chatKey, (old) =>
        old && !old.messages.some((m) => m.id === msg.id) ? { ...old, messages: [...old.messages, msg] } : old,
      );

    let stopped = false;
    let retries = 0;
    let connectedBefore = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const controller = new AbortController();

    async function connect() {
      try {
        const res = await openChatStream(ticketId, controller.signal);
        if (!res.ok || !res.body) throw new Error(`Chat stream failed (${res.status})`);
        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';
        for (;;) {
          const { value, done } = await reader.read();
          if (done) break;
          buffer += decoder.decode(value, { stream: true });
          const parsed = parseSSE(buffer);
          buffer = parsed.rest;
          for (const ev of parsed.events) {
            if (ev.event === 'ready') {
              setStatus('live');
              retries = 0;
              if (connectedBefore) queryClient.invalidateQueries({ queryKey: chatKey }); // catch up after a gap
              connectedBefore = true;
            } else if (ev.event === 'message') {
              append(JSON.parse(ev.data) as ChatMessage);
            }
          }
        }
        throw new Error('Chat stream closed');
      } catch {
        if (stopped) return;
        setStatus('reconnecting');
        timer = setTimeout(connect, Math.min(MAX_RETRY_DELAY_MS, 1000 * 2 ** retries++));
      }
    }

    connect();
    return () => {
      stopped = true;
      controller.abort();
      clearTimeout(timer);
    };
  }, [ticketId, queryClient]);

  const send = useMutation({
    mutationFn: (body: string) => chatApi.send(ticketId, body),
    onSuccess: (msg) =>
      queryClient.setQueryData<ChatThread>(['chat', ticketId], (old) =>
        old && !old.messages.some((m) => m.id === msg.id) ? { ...old, messages: [...old.messages, msg] } : old,
      ),
  });

  return { thread, status, send };
}
