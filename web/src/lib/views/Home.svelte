<script lang="ts">
  import { traceDot } from '../perf';
  import { api } from '../api';
  import { auth } from '../auth.svelte';
  import { costLabel, total } from '../costs';
  import { cacheHitRate, formatPercent, formatTime, formatTokens, totalTokens } from '../format';
  import { hrefFor } from '../router.svelte';
  import AccountCard from '../components/AccountCard.svelte';
  import LimitCard from '../components/LimitCard.svelte';
  import StatusDot from '../components/StatusDot.svelte';
  import type { Account } from '../types/accounts';
  import type { CostReport, Redaction, ServiceHealth, Status, TraceSummary } from '../types/dashboard';

  let status = $state<Status | undefined>();
  let report = $state<CostReport | undefined>();
  let traces = $state<TraceSummary[] | undefined>();
  let redactions = $state<Redaction[] | undefined>();
  let subscriptions = $state<Account[] | undefined>();
  let errors = $state<{ status?: string; costs?: string; traces?: string; redactions?: string; accounts?: string }>({});
  import TrendCharts from '../components/TrendCharts.svelte';
  import CallRate from '../components/CallRate.svelte';
  import { isFull, orderSubscriptions, switchCandidate } from '../subscriptions';
  import type { Agg } from '../costs';
  import { deltaLabel, pointsLabel, weekComparison } from '../windows';
  // A full active account is not switched automatically; the next free one gets a button (#126).
  const activeFull = $derived(!!subscriptions?.some((a) => a.active && isFull(a, Date.now())));
  const candidate = $derived(subscriptions ? switchCandidate(subscriptions, Date.now()) : undefined);
  let switching = $state(false);
  let switched = $state<{ ok: boolean; text: string; notice: string } | undefined>();
  import type { SwitchStatus } from '../types/accounts';
  let switchInfo = $state<SwitchStatus | undefined>();
  $effect(() => {
    if (!switching) return;
    let stale = false;
    const poll = () => { void api.accountSwitchStatus().then((s) => { if (!stale) switchInfo = s; }).catch(() => {}); };
    poll();
    const id = setInterval(poll, 1000);
    return () => { stale = true; clearInterval(id); switchInfo = undefined; };
  });
  // Aborts the running requests so the reserved switch runs at once (#142).
  async function forceSwitch() {
    try { switchInfo = await api.forceAccountSwitch(); } catch (e) { switched = { ok: false, text: (e as Error).message, notice: '' }; }
  }
  async function switchTo(a: Account) {
    switching = true;
    switched = undefined;
    try {
      const r = await api.activateAccount(a.id);
      const name = r.account.email || r.account.name;
      switched = { ok: r.test.ok, text: r.test.ok ? `${name} ist jetzt aktiv.` : `${name} ist jetzt aktiv, der Test schlug fehl: ${r.test.message}`, notice: r.cache_notice };
      await load();
    } catch (e) {
      switched = { ok: false, text: (e as Error).message, notice: '' };
    } finally {
      switching = false;
    }
  }
  // Bumped on every refresh so the 7-day charts reload with the page (#123).
  let trendTick = $state(0);

  // Same cards as Accounts: the active subscription first, then who can take
  // over when its window is full (#124). Usage is cached server-side.
  async function loadSubscriptions() {
    try {
      const list = await api.listAccounts();
      const usage = await Promise.allSettled(list.map((a) => api.accountUsage(a.id)));
      subscriptions = orderSubscriptions(
        list.map((a, i) => { const u = usage[i]; return u.status === 'fulfilled' ? { ...a, usage: u.value } : a; }),
        Date.now(),
      );
      errors.accounts = undefined;
    } catch (e) {
      errors.accounts = (e as Error).message;
    }
  }

  // Each section loads on its own, so one failing API does not hide the others.
  async function load() {
    const [s, c, t, r] = await Promise.allSettled([api.status(), api.costs(14), api.traces({ limit: 10 }), api.redactions(10)]);
    const msg = (r: PromiseSettledResult<unknown>) => (r.status === 'rejected' ? (r.reason as Error).message : undefined);
    errors = { status: msg(s), costs: msg(c), traces: msg(t), redactions: msg(r) };
    if (s.status === 'fulfilled') status = s.value;
    if (status?.account_management) await loadSubscriptions();
    if (c.status === 'fulfilled') report = c.value;
    if (t.status === 'fulfilled') traces = t.value.traces;
    if (r.status === 'fulfilled') redactions = r.value.redactions;
  }

  $effect(() => {
    if (!auth.token) return;
    void load();
    const id = setInterval(() => {
      void load();
      trendTick++;
    }, 30_000);
    return () => clearInterval(id);
  });

  // Fourteen days cover this calendar week and the whole last week (#125).
  const today = $derived(report ? total(report.rows.filter((r) => r.day === report!.to)) : undefined);
  const week = $derived(report ? weekComparison(report.rows, report.to) : undefined);
  const tokensIn = (a: Agg) => a.usage.input + a.usage.cache_read + a.usage.cache_creation;
  const accounts = $derived(status?.accounts ?? []);
  const active = $derived(accounts.length === 1 ? accounts[0] : undefined);
  const services = $derived<Array<[string, ServiceHealth]>>(status ? [['tollgate', status.tollgate], ['CLIProxyAPI', status.cliproxyapi]] : []);

  function accountDot(): { state: 'ok' | 'warn' | 'idle'; label: string; text: string } {
    if (accounts.length > 1) return { state: 'warn', label: 'unklar', text: `unklar: ${accounts.length} Accounts im auth-dir` };
    if (!active) return { state: 'idle', label: 'unbekannt', text: 'unbekannt' };
    const text = active.email || active.name;
    if (active.disabled) return { state: 'idle', label: 'deaktiviert', text };
    if (active.cooldown_until) return { state: 'warn', label: 'Cooldown', text };
    return { state: 'ok', label: 'aktiv', text };
  }

</script>

<div class="page-head">
  <h1>Übersicht</h1>
  <button type="button" class="btn" onclick={() => { void load(); trendTick++; }}>Aktualisieren</button>
</div>

<section aria-labelledby="h-limits">
  <h2 id="h-limits" class="section first">Abo-Limits</h2>
  {#if errors.status}<p class="err" role="alert">{errors.status}</p>{/if}
  {#if status}
    {#if status.account_management}
      {#if errors.accounts}<p class="err" role="alert">{errors.accounts}</p>{/if}
      {#if subscriptions}
        <div class="subscriptions">
          {#each subscriptions as account (account.id)}
            {@const signal = account.active && activeFull ? 'full' : account.id === candidate?.id ? 'next' : undefined}
            {#if signal === 'next'}
              <AccountCard {account} hours={report?.hours} {signal}>
                <button type="button" class="btn primary" disabled={switching} onclick={() => switchTo(account)}>{switching ? 'Wechsle …' : 'Account wechseln'}</button>
                {#if switching && switchInfo?.waiting && switchInfo.running_requests > 0}
                  <button type="button" class="btn" onclick={forceSwitch} title="Bricht {switchInfo.running_requests} laufende Anfragen ab; Claude Code wiederholt sie.">Sofort wechseln</button>
                {/if}
              </AccountCard>
            {:else}
              <AccountCard {account} hours={report?.hours} {signal} />
            {/if}
          {/each}
        </div>
        {#if switched}
          <p class:err={!switched.ok} class:muted={switched.ok} role="status">{switched.text}{#if switched.notice}{' '}<span class="muted">{switched.notice}</span>{/if}</p>
        {/if}
        {#if subscriptions.length === 0}<p class="muted">Noch kein Abo gespeichert.</p>{/if}
      {:else if !errors.accounts}<p class="muted">Lade …</p>{/if}
    {:else}
      <LimitCard title={active ? active.email || active.name : 'Aktives Abo'} limits={status.limits} active={!!active} />
    {/if}
  {:else if !errors.status}
    <p class="muted">Lade …</p>
  {/if}
</section>

<section aria-label="Letzte 5 Minuten">
  <h2 class="section">Letzte 5 Minuten <span class="muted small">(gegen die 10 Fenster davor)</span></h2>
  <CallRate refresh={trendTick} />
</section>

<section aria-label="Verlauf">
  <h2 class="section">Verlauf <span class="muted small">(7 Tage, UTC)</span></h2>
  <TrendCharts refresh={trendTick} names={new Map((subscriptions ?? []).map((a) => [a.id, a.name]))} />
</section>

<section aria-label="Heute">
  <h2 class="section">Heute <span class="muted small">(UTC)</span></h2>
  {#if errors.costs}<p class="err" role="alert">{errors.costs}</p>{/if}
  {#if today}
    <dl class="tiles">
      <div><dt>Anfragen</dt><dd>{formatTokens(today.requests)}</dd></div>
      <div><dt>Tokens ein</dt><dd>{formatTokens(today.usage.input + today.usage.cache_read + today.usage.cache_creation)}</dd></div>
      <div><dt>Tokens aus</dt><dd>{formatTokens(today.usage.output)}</dd></div>
      <div><dt>Cache-Trefferquote</dt><dd>{formatPercent(cacheHitRate(today.usage))}</dd></div>
      <div><dt>Gespart</dt><dd>{formatTokens(today.saved_tokens)}</dd></div>
      <div><dt>Kosten</dt><dd>{costLabel(today)}</dd></div>
    </dl>
  {:else if !errors.costs}
    <p class="muted">Lade …</p>
  {/if}
</section>

<section aria-label="Woche">
  <h2 class="section">Diese Woche <span class="muted small">(seit Montag, UTC, gegen Vorwoche bis zum selben Wochentag)</span></h2>
  {#if errors.costs}<p class="err" role="alert">{errors.costs}</p>{/if}
  {#if week}
    {@const c = week.current}
    {@const p = week.previous}
    {#snippet tile(label: string, value: string, prev: string, delta: string | null)}
      <div><dt>{label}</dt><dd>{value}</dd><p class="vs">Vorwoche {prev}{#if delta}{' · '}<span class="delta">{delta}</span>{/if}</p></div>
    {/snippet}
    <dl class="tiles">
      {@render tile('Kosten', costLabel(c), costLabel(p), deltaLabel(c.cost_nano_usd, p.cost_nano_usd))}
      {@render tile('Anfragen', formatTokens(c.requests), formatTokens(p.requests), deltaLabel(c.requests, p.requests))}
      {@render tile('Tokens ein', formatTokens(tokensIn(c)), formatTokens(tokensIn(p)), deltaLabel(tokensIn(c), tokensIn(p)))}
      {@render tile('Tokens aus', formatTokens(c.usage.output), formatTokens(p.usage.output), deltaLabel(c.usage.output, p.usage.output))}
      {@render tile('Cache-Trefferquote', formatPercent(cacheHitRate(c.usage)), formatPercent(cacheHitRate(p.usage)), pointsLabel(cacheHitRate(c.usage), cacheHitRate(p.usage)))}
      {@render tile('Gespart', formatTokens(c.saved_tokens), formatTokens(p.saved_tokens), deltaLabel(c.saved_tokens, p.saved_tokens))}
    </dl>
    <table class="week">
      <caption class="muted small">Pro Tag; Vorwoche gesamt {costLabel(week.previous_full)} · {formatTokens(totalTokens(week.previous_full.usage))} Tokens</caption>
      <thead><tr><th scope="col">Tag</th><th scope="col">Kosten</th><th scope="col">Tokens</th><th scope="col">Vorwoche</th><th scope="col">Tokens Vorwoche</th><th scope="col">Änderung</th></tr></thead>
      <tbody>
        {#each week.days as d (d.day)}
          <tr class:future={!d.current}>
            <th scope="row">{d.weekday} <span class="muted">{d.day.slice(8, 10)}.{d.day.slice(5, 7)}.</span></th>
            <td>{d.current ? costLabel(d.current) : '–'}</td>
            <td>{d.current ? formatTokens(totalTokens(d.current.usage)) : '–'}</td>
            <td>{costLabel(d.previous)}</td>
            <td>{formatTokens(totalTokens(d.previous.usage))}</td>
            <td>{(d.current && deltaLabel(d.current.cost_nano_usd, d.previous.cost_nano_usd)) ?? '–'}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  {:else if !errors.costs}
    <p class="muted">Lade …</p>
  {/if}
</section>

<div class="cols">
  <section aria-labelledby="h-health">
    <h2 id="h-health" class="section">Health</h2>
    {#if errors.status}<p class="err" role="alert">{errors.status}</p>{/if}
    {#if status}
      {@const acc = accountDot()}
      <ul class="health">
        {#each services as [label, h] (label)}
          <li>
            <StatusDot state={h.ok ? 'ok' : 'error'} label={h.ok ? 'in Ordnung' : 'gestört'} />
            <strong>{label}</strong>
            <span class="muted">{[h.version, h.message].filter(Boolean).join(' · ') || (h.ok ? 'in Ordnung' : 'gestört')}</span>
          </li>
        {/each}
        <li>
          <StatusDot state={acc.state} label={acc.label} />
          <strong>Account</strong>
          <span class="muted">{acc.text}</span>
        </li>
      </ul>
    {/if}
  </section>

  <section aria-labelledby="h-recent">
    <div class="sec-head">
      <h2 id="h-recent" class="section">Letzte Anfragen</h2>
      <a class="more" href={hrefFor('live')}>Alle in Live</a>
    </div>
    {#if errors.traces}<p class="err" role="alert">{errors.traces}</p>{/if}
    {#if traces && traces.length === 0}
      <p class="muted">Noch keine Anfragen.</p>
    {:else if traces}
      <ul class="recent">
        {#each traces as t (t.id)}
          {@const d = traceDot(t)}
          <li>
            <a href={hrefFor('live', t.id)}>
              <StatusDot state={d.state} label={d.label} />
              <span class="time num">{formatTime(t.started_at)}</span>
              <span class="client">{t.client_name || '–'}</span>
              <span class="model mono">{t.model || '–'}</span>
              <span class="tok num">{t.usage ? formatTokens(totalTokens(t.usage)) : '–'}</span>
            </a>
          </li>
        {/each}
      </ul>
    {/if}
  </section>
</div>
<section aria-labelledby="h-redactions">
  <h2 id="h-redactions" class="section">Letzte Redactions</h2>
  {#if errors.redactions}<p class="err" role="alert">{errors.redactions}</p>{/if}
  {#if redactions && redactions.length === 0}
    <p class="muted">Noch keine Redactions.</p>
  {:else if redactions}
    <!-- Type, placeholder, mask (computed by the redact stage, issue #112) and rule: the replaced value never reaches the frontend. -->
    <ul class="recent red">
      {#each redactions as r, i (i)}
        <li>
          <a href={hrefFor('live', r.trace_id)} title="Anfrage in Live öffnen">
            <span class="time num">{formatTime(r.at)}</span>
            <span class="client">{r.client_name || '–'}</span>
            <span class="type">{r.type}</span>
            <span class="ph mono">{r.placeholder || '–'}</span>
            <span class="mask mono" title="Maske">{r.mask || '–'}</span>
            <span class="rule muted">{#if r.rule !== r.type}{r.rule}{/if}</span>
          </a>
        </li>
      {/each}
    </ul>
  {/if}
</section>

<style>
  .first { margin-top: 0; }
  .subscriptions { display: grid; gap: 0.9rem; grid-template-columns: repeat(auto-fit, minmax(min(100%, 20rem), 1fr)); }
  .small { font-size: 0.8rem; font-weight: 400; }
  .tiles { display: grid; grid-template-columns: repeat(auto-fit, minmax(8.5rem, 1fr)); gap: 0.75rem; margin: 0; }
  .tiles div { padding: 0.8rem 1rem; background: var(--c-sidebar); border-radius: var(--radius); }
  dt { font-size: 0.8rem; color: var(--c-muted); }
  dd { margin: 0; font-size: 1.35rem; font-weight: 600; font-variant-numeric: tabular-nums; }
  .vs { margin: 0.2rem 0 0; font-size: 0.75rem; color: var(--c-muted); font-variant-numeric: tabular-nums; }
  .delta { color: var(--c-text); }
  .week { width: 100%; margin-top: 0.9rem; border-collapse: collapse; font-size: 0.88rem; font-variant-numeric: tabular-nums; }
  .week caption { caption-side: bottom; text-align: left; padding-top: 0.4rem; }
  .week th, .week td { padding: 0.35rem 0.5rem; text-align: right; border-bottom: 1px solid var(--c-line); white-space: nowrap; }
  .week th:first-child { text-align: left; font-weight: 500; }
  .week thead th { font-size: 0.78rem; font-weight: 500; color: var(--c-muted); }
  .week .future td { color: var(--c-muted); }
  .cols { display: grid; grid-template-columns: minmax(0, 2fr) minmax(0, 3fr); gap: 0 2rem; }
  .health { list-style: none; margin: 0; padding: 0; display: grid; gap: 0.6rem; }
  .health li { display: flex; align-items: center; gap: 0.6rem; flex-wrap: wrap; min-width: 0; }
  .sec-head { display: flex; align-items: baseline; justify-content: space-between; gap: 1rem; }
  .more { font-size: 0.85rem; color: var(--c-accent); text-decoration: none; }
  .more:hover { text-decoration: underline; }
  .recent { list-style: none; margin: 0; padding: 0; }
  .recent a { display: grid; grid-template-columns: 0.6rem 4.5rem 1fr 1.2fr 4.5rem; align-items: center; gap: 0.6rem; padding: 0.45rem 0.4rem; border-radius: var(--radius); color: inherit; text-decoration: none; font-size: 0.88rem; }
  .recent a:hover { background: var(--c-hover); }
  .recent a:focus-visible, .more:focus-visible { outline: 2px solid var(--c-accent); outline-offset: -2px; }
  .recent span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .time { color: var(--c-muted); }
  .red a { grid-template-columns: 4.5rem 1fr 7rem 1.6fr 1.4fr 1fr; }
  .type { color: var(--c-accent); }
  @media (max-width: 900px) { .cols { grid-template-columns: 1fr; } }
  @media (max-width: 720px) {
    .recent a { grid-template-columns: 0.6rem 1fr 4.5rem; }
    .red a { grid-template-columns: minmax(0, 1.3fr) minmax(0, 1fr); }
    .red .type { display: none; }
    .time, .model, .red .rule, .red .client { display: none; }
    .week th:nth-child(3), .week td:nth-child(3), .week th:nth-child(5), .week td:nth-child(5) { display: none; }
  }
</style>
