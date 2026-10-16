// Small typed client for the dashboard and client admin APIs.
import type { CostReport, PerfReport, RedactionList, SpeedtestResponse, Status, TraceDetail, TraceList } from './types/dashboard';
import type { DryRunResult, Issue, TestResult as FilterTestResult, VersionDetail, VersionInfo, VersionList, WordOps } from './types/redactcfg';
import type { ClientInfo, CreateClientResponse, ListClientsResponse, ClientResponse } from './types/auth';
import type { Account, UsageSnapshot, SwitchStatus, ListResponse, Login, Activation, TestResult } from './types/accounts';
import { auth, logout } from './auth.svelte';

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
    /** Validation issues of a 422 from /api/config. */
    public issues: Issue[] = [],
  ) {
    super(message);
  }
}

export interface ApiOptions {
  /** Injectable for tests; defaults to the global fetch. */
  fetch?: typeof fetch;
  token: () => string | undefined;
  onUnauthorized?: () => void;
}

export interface TraceQuery {
  limit?: number;
  before?: string;
  before_id?: string;
}

export function createApi(o: ApiOptions) {
  const f: typeof fetch = (...a) => (o.fetch ?? fetch)(...a);

  function headers(extra?: Record<string, string>): Record<string, string> {
    const t = o.token();
    return { ...(t ? { Authorization: `Bearer ${t}` } : {}), ...extra };
  }

  async function send(method: string, path: string, body?: unknown): Promise<Response> {
    const res = await f(path, {
      method,
      headers: headers(body === undefined ? undefined : { 'Content-Type': 'application/json' }),
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    if (res.status === 401) {
      o.onUnauthorized?.();
      throw new ApiError(401, 'Token abgelehnt');
    }
    if (!res.ok) {
      let msg = `Fehler ${res.status}`;
      let issues: Issue[] = [];
      try {
        const j = (await res.json()) as { error?: string; issues?: Issue[] };
        if (j.error) msg = j.error;
        issues = j.issues ?? [];
      } catch {
        // keep the generic message
      }
      throw new ApiError(res.status, msg, issues);
    }
    return res;
  }

  async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
    return (await (await send(method, path, body)).json()) as T;
  }

  const enc = encodeURIComponent;
  return {
    /** Headers for non-JSON requests such as the SSE stream. */
    headers,
    traces(q: TraceQuery = {}): Promise<TraceList> {
      const p = new URLSearchParams();
      if (q.limit) p.set('limit', String(q.limit));
      if (q.before) p.set('before', q.before);
      if (q.before_id) p.set('before_id', q.before_id);
      const s = p.toString();
      return request('GET', `/api/traces${s ? `?${s}` : ''}`);
    },
    costs: (days: number): Promise<CostReport> => request('GET', `/api/costs?days=${days}`),
    performance: (days: number): Promise<PerfReport> => request('GET', `/api/performance?days=${days}`),
    /** Runs real requests (subscription quota); confirm must come from the user. */
    speedtest: (models: string[]): Promise<SpeedtestResponse> => request('POST', '/api/speedtest', { models, confirm: true }),
    traceDetail: (id: string): Promise<TraceDetail> => request('GET', `/api/traces/${enc(id)}`),
    redactions: (limit: number): Promise<RedactionList> => request('GET', `/api/redactions?limit=${limit}`),
    async configSchema(): Promise<unknown> {
      return (await send('GET', '/api/config/schema')).json();
    },
    configVersions: (): Promise<VersionList> => request('GET', '/api/config/versions'),
    configVersion: (id: number): Promise<VersionDetail> => request('GET', `/api/config/versions/${id}`),
    activateConfig: (id: number): Promise<VersionInfo> => request('POST', `/api/config/versions/${id}/activate`),
    rollbackConfig: (id: number): Promise<VersionInfo> => request('POST', `/api/config/versions/${id}/rollback`),
    /** ops edit the word list on the server (words_from or draft list); it never leaves the server. */
    dryRun: (yaml: string, words_from: number, ops: WordOps = {}): Promise<DryRunResult> => request('POST', '/api/config/dry-run', { yaml, words_from, ...ops }),
    importConfig: (yaml: string, words_from: number, comment: string, activate: boolean, ops: WordOps = {}): Promise<VersionInfo> =>
      request('POST', '/api/config/import', { yaml, words_from, comment, activate, ...ops }),
    testFilter: (text: string, yaml: string, words_from: number, ops: WordOps = {}): Promise<FilterTestResult> =>
      request('POST', '/api/config/test', { text, version: 0, yaml, words_from, ...ops }),
    /** Plain text of the three name lists of a version (ADR-0021): only on an explicit click, keep it only while editing. */
    nameLists: (version: number): Promise<import('./types/redactcfg').NameLists> => request('GET', `/api/config/lists?version=${version}`),
    /** YAML export of the active version; words=plain includes the word lists. */
    async exportConfig(words: 'omit' | 'plain'): Promise<{ text: string; filename: string }> {
      const res = await send('GET', `/api/config/export?words=${words}`);
      const m = /filename="([^"]+)"/.exec(res.headers.get('Content-Disposition') ?? '');
      return { text: await res.text(), filename: m?.[1] ?? 'tollgate-config.yaml' };
    },
    accountSwitchStatus(): Promise<SwitchStatus> { return request('GET', '/api/accounts/switch-status'); },
    forceAccountSwitch: (): Promise<SwitchStatus> => request('POST', '/api/accounts/switch/force'),
    accountUsage(id: string): Promise<UsageSnapshot> { return request('GET', `/api/accounts/${enc(id)}/usage`); },
    async listAccounts(): Promise<Account[]> { return (await request<ListResponse>('GET', '/api/accounts')).accounts; },
    startAccountLogin: (): Promise<Login> => request('POST', '/api/accounts/login'),
    completeAccountLogin: (id: string, callback_url: string, name: string): Promise<Account> => request('POST', '/api/accounts/login/complete', { id, callback_url, name }),
    importAccount: (name: string, token: unknown): Promise<Account> => request('POST', '/api/accounts/import', { name, token }),
    activateAccount: (id: string): Promise<Activation> => request('POST', `/api/accounts/${enc(id)}/activate`),
    testAccount: (id: string): Promise<TestResult> => request('POST', `/api/accounts/${enc(id)}/test`),
    removeAccount: (id: string): Promise<{ ok: boolean }> => request('DELETE', `/api/accounts/${enc(id)}`),
    status: (): Promise<Status> => request('GET', '/api/status'),
    /** Requests of the last five minutes against the ten windows before (#132). */
    calls: (): Promise<import('./types/dashboard').CallRate> => request('GET', '/api/calls'),
    async listClients(): Promise<ClientInfo[]> {
      return (await request<ListClientsResponse>('GET', '/api/clients')).clients;
    },
    createClient: (name: string): Promise<CreateClientResponse> => request('POST', '/api/clients', { name }),
    async blockClient(id: string): Promise<ClientInfo> {
      return (await request<ClientResponse>('POST', `/api/clients/${enc(id)}/block`)).client;
    },
    rotateClient: (id: string): Promise<CreateClientResponse> => request('POST', `/api/clients/${enc(id)}/rotate`),
    compressRules: (): Promise<import('./types/compress').RuleList> => request('GET', '/api/compress/rules'),
    /** Runs one command and its output through the compress stage of the active version (#21). */
    compressPlayground: (command: string, output: string): Promise<import('./types/compress').Explanation> =>
      request('POST', '/api/compress/playground', { command, output, version: 0 }),
  };
}

export const api = createApi({ token: () => auth.token, onUnauthorized: () => logout(true) });
