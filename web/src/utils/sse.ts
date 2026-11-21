export interface SSEEvent {
  event: string;
  data: string;
}

/**
 * Parses complete Server-Sent Events from a text buffer.
 * Returns the parsed events and the unfinished remainder to keep for the next chunk.
 */
export function parseSSE(buffer: string): { events: SSEEvent[]; rest: string } {
  const blocks = buffer.split(/\r?\n\r?\n/);
  const rest = blocks.pop() ?? '';
  const events: SSEEvent[] = [];
  for (const block of blocks) {
    let event = 'message';
    const data: string[] = [];
    for (const line of block.split(/\r?\n/)) {
      if (line.startsWith(':')) continue; // comment / keep-alive
      if (line.startsWith('event:')) event = line.slice(6).trim();
      else if (line.startsWith('data:')) data.push(line.slice(5).replace(/^ /, ''));
    }
    if (data.length > 0 || event !== 'message') events.push({ event, data: data.join('\n') });
  }
  return { events, rest };
}
