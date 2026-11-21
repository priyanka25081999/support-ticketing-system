import { parseSSE } from '../utils/sse';
import { readRange, resolveRange, shiftRange, writeRange } from '../utils/timeRange';

describe('parseSSE', () => {
  it('parses complete events and keeps the unfinished remainder', () => {
    const { events, rest } = parseSSE('event: ready\ndata: {}\n\n: ping\n\nevent: message\ndata: {"id":"1"}\n\nevent: mess');
    expect(events).toEqual([
      { event: 'ready', data: '{}' },
      { event: 'message', data: '{"id":"1"}' },
    ]);
    expect(rest).toBe('event: mess');
  });
});

describe('time range', () => {
  const now = Date.parse('2026-11-21T12:00:00Z');

  it('resolves presets relative to now', () => {
    expect(resolveRange({ preset: '24h' }, now)).toEqual({ from: '2026-11-20T12:00:00.000Z' });
    expect(resolveRange({ preset: 'all' }, now)).toEqual({});
  });

  it('round-trips through URL params', () => {
    const abs = { from: '2026-11-01T00:00:00.000Z', to: '2026-11-02T00:00:00.000Z' };
    expect(readRange(writeRange(new URLSearchParams(), abs, '30d'), '30d')).toEqual(abs);
    expect(readRange(writeRange(new URLSearchParams(), { preset: '7d' }, '30d'), '30d')).toEqual({ preset: '7d' });
    expect(writeRange(new URLSearchParams('range=7d'), { preset: '30d' }, '30d').toString()).toBe('');
    expect(readRange(new URLSearchParams('range=bogus'), '30d')).toEqual({ preset: '30d' });
  });

  it('shifts a range back by its own length and never into the future', () => {
    expect(shiftRange({ preset: '24h' }, -1, now)).toEqual({ from: '2026-11-19T12:00:00.000Z', to: '2026-11-20T12:00:00.000Z' });
    expect(shiftRange({ preset: '24h' }, 1, now)).toEqual({ from: '2026-11-20T12:00:00.000Z', to: '2026-11-21T12:00:00.000Z' });
    expect(shiftRange({ preset: 'all' }, -1, now)).toBeNull();
  });
});
