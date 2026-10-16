import { describe, expect, it, vi } from 'vitest';
import { ApiError, createApi } from './api';
import { json } from '../test-helpers';

function setup(res: Response | (() => Response), token: string | undefined) {
  const f = vi.fn(async (_url: string, _init?: RequestInit) => (typeof res === 'function' ? res() : res));
  const onUnauthorized = vi.fn();
  const api = createApi({ fetch: f as unknown as typeof fetch, token: () => token, onUnauthorized });
  return { f, api, onUnauthorized };
}
const call = (f: { mock: { calls: Array<[string, RequestInit?]> } }) => ({ url: f.mock.calls[0][0], init: f.mock.calls[0][1] as RequestInit });

describe('api client', () => {
  it('sends the admin token as Bearer header and never in the URL', async () => {
    const { f, api } = setup(json({ traces: [], next_before: '', next_before_id: '' }), 'secret');
    await api.traces({ limit: 50, before: '2026-10-04T10:00:00Z', before_id: 'x' });
    const { url, init } = call(f);
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer secret');
    expect(url).toBe('/api/traces?limit=50&before=2026-10-04T10%3A00%3A00Z&before_id=x');
    expect(url).not.toContain('secret');
  });

  it('calls costs, status and the client endpoints', async () => {
    const { f, api } = setup(() => json({ clients: [], client: { id: 'a b' }, key: 'k' }), 'secret');
    await api.costs(7);
    await api.status();
    await api.listClients();
    await api.createClient('x');
    await api.blockClient('a b');
    await api.rotateClient('a b');
    const calls = f.mock.calls.map((c) => `${(c[1] as RequestInit).method} ${c[0]}`);
    expect(calls).toEqual([
      'GET /api/costs?days=7',
      'GET /api/status',
      'GET /api/clients',
      'POST /api/clients',
      'POST /api/clients/a%20b/block',
      'POST /api/clients/a%20b/rotate',
    ]);
    expect(JSON.parse(f.mock.calls[3][1]!.body as string)).toEqual({ name: 'x' });
  });

  it('reports 401 via onUnauthorized and throws', async () => {
    const { api, onUnauthorized } = setup(json({ error: 'unauthorized' }, 401), 'secret');
    await expect(api.status()).rejects.toMatchObject({ status: 401 });
    expect(onUnauthorized).toHaveBeenCalledOnce();
  });

  it('uses the error message of the body for other failures', async () => {
    const { api, onUnauthorized } = setup(json({ error: 'name must be 1 to 128 characters' }, 400), 'secret');
    const err = await api.createClient('').catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.message).toBe('name must be 1 to 128 characters');
    expect(onUnauthorized).not.toHaveBeenCalled();
  });

  it('omits the header without a token', async () => {
    const { f, api } = setup(json({}), undefined);
    await api.status();
    expect((call(f).init.headers as Record<string, string>).Authorization).toBeUndefined();
  });
});
