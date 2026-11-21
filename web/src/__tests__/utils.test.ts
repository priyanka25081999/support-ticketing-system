import { getErrorMessage, humanize, nextStatuses } from '../utils';

describe('getErrorMessage', () => {
  it('uses the API error message when present', () => {
    const err = { response: { status: 409, data: { error: { code: 'CONFLICT', message: 'Already linked' } } } };
    expect(getErrorMessage(err)).toBe('Already linked');
  });

  it('explains network failures', () => {
    expect(getErrorMessage({ request: {}, message: 'Network Error' })).toMatch(/Cannot reach the server/);
  });

  it('hides raw 5xx details', () => {
    expect(getErrorMessage({ response: { status: 502, data: '<html>' } })).toMatch(/Server error/);
  });
});

describe('nextStatuses', () => {
  it('lets admins close solved tickets but not agents', () => {
    expect(nextStatuses('admin', 'solved')).toEqual(['open', 'closed']);
    expect(nextStatuses('agent', 'solved')).toEqual(['open']);
  });

  it('only lets customers reopen solved tickets', () => {
    expect(nextStatuses('customer', 'open')).toEqual([]);
    expect(nextStatuses('customer', 'solved')).toEqual(['open']);
  });

  it('treats closed as final', () => {
    expect(nextStatuses('admin', 'closed')).toEqual([]);
  });
});

test('humanize formats enum values', () => {
  expect(humanize('feature_request')).toBe('Feature request');
});
