import { beforeEach, describe, expect, it, vi } from 'vitest';

const httpRequest = vi.hoisted(() => vi.fn());
vi.mock('@/api/http-init', () => ({
  httpRequest,
  HttpError: class HttpError extends Error {
    status = 0;
    response = { status: 0, statusText: '', data: null };
  },
}));
vi.mock('@/utils/messageBus', () => ({
  getMessage: () => ({
    success: vi.fn(),
    error: vi.fn(),
    warning: vi.fn(),
    info: vi.fn(),
    loading: vi.fn(),
  }),
}));

import { postSubConv } from '@/api/queries/useSubConvWorkbench';

// The subconv write handlers bind nested request structs from JSON only —
// a form-encoded body flattens them into empty models and the backend then
// reports "publication name is required" for a filled-in form.
describe('postSubConv', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    httpRequest.mockResolvedValue({
      ok: true,
      status: 200,
      statusText: 'OK',
      data: { success: true, msg: 'ok', obj: {} },
    });
  });

  it('sends a nested publication payload as JSON, not as flattened form fields', async () => {
    const payload = {
      publication: { id: '', name: 'pub', sourceIds: [], subIds: ['s1'], enable: true },
      rotateToken: true,
    };

    await postSubConv('/panel/api/subconv/publication/save', payload);

    const [method, url, data, options] = httpRequest.mock.calls[0] as [
      string,
      string,
      unknown,
      { headers?: Record<string, string> },
    ];
    expect(method).toBe('POST');
    expect(url).toBe('/panel/api/subconv/publication/save');
    expect(options.headers?.['Content-Type']).toMatch(/^application\/json/);
    expect(data).toEqual(payload);
  });
});
