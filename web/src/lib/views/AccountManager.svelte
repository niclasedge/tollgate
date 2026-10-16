<script lang="ts">
  import { api } from '../api';
 import { tick, untrack } from 'svelte';
  import { auth } from '../auth.svelte';
  import { formatDateTime } from '../format';
  import StatusDot from '../components/StatusDot.svelte';
 import AccountCard from '../components/AccountCard.svelte';
  import { TokenRelogin } from '../types/accounts';
  import type { Account, Login, TestResult, SwitchStatus } from '../types/accounts';

  let { onchange = () => {} }: { onchange?: () => void } = $props();
  let accounts = $state<Account[]>([]);
  let loaded = $state(false);
  let busy = $state(false);
  let error = $state('');
  let message = $state('');
  let login = $state<Login | undefined>();
  let name = $state('');
  let callback = $state('');
  let pending = $state<{ account: Account; action: 'activate' | 'remove' } | undefined>();
  let testResult = $state<TestResult | undefined>();
  let copied = $state(false);
 let waitingSwitch = $state(false);
 let switchTarget = $state('');
 let switchInfo = $state<SwitchStatus | undefined>();
 let forcing = $state(false);
 let usageLoading = $state<Record<string, boolean>>({});
 let loadGeneration = 0;

  function focusOnMount(node: HTMLElement) { node.focus(); }

  async function load() {
    const generation = ++loadGeneration;
    const previous = untrack(() => new Map(accounts.map((a) => [a.id, a.usage])));
    accounts = (await api.listAccounts()).map((a) => ({ ...a, usage: previous.get(a.id) }));
    loaded = true;
    for (const account of accounts) {
      usageLoading[account.id] = true;
      void api.accountUsage(account.id).then((usage) => {
        if (generation !== loadGeneration) return;
        accounts = accounts.map((a) => a.id === account.id ? { ...a, usage } : a);
      }).catch(() => {
        if (generation !== loadGeneration) return;
        accounts = accounts.map((a) => a.id === account.id ? { ...a, usage: { ...(a.usage ?? { five_hour: { utilization: null, reset: '' }, seven_day: { utilization: null, reset: '' }, observed_at: '' }), error: 'Limits momentan nicht abrufbar' } } : a);
      }).finally(() => { if (generation === loadGeneration) usageLoading[account.id] = false; });
    }
  }
  $effect(() => {
    if (!auth.token) return;
    void load().catch((e: Error) => { error = e.message; });
    const id = setInterval(() => { if (!busy) void load().catch((e: Error) => { error = e.message; }); }, 30_000);
    return () => clearInterval(id);
  });
  $effect(() => {
    if (!waitingSwitch) return;
    let stale = false;
    const poll = () => { void api.accountSwitchStatus().then((s) => { if (!stale) switchInfo = s; }).catch(() => {}); };
    poll();
    const id = setInterval(poll, 1000);
    return () => { stale = true; clearInterval(id); };
  });
  // Aborts the running requests so the reserved switch runs at once (#142).
  async function forceSwitch() {
    forcing = true;
    try { switchInfo = await api.forceAccountSwitch(); } catch (e) { error = (e as Error).message; }
    finally { forcing = false; }
  }
  async function run(fn: () => Promise<void>) {
    if (busy) return;
    busy = true; error = ''; message = ''; testResult = undefined;
    try { await fn(); } catch (e) { error = (e as Error).message; }
    finally { busy = false; }
  }
  const start = (prefill = '') => run(async () => { login = await api.startAccountLogin(); name = prefill; callback = ''; copied = false; pending = undefined; });
  // Closing the inline confirmation re-renders the actions; focus the trigger again.
  async function cancelPending() {
    const current = pending;
    pending = undefined;
    if (!current) return;
    await tick();
    document.getElementById(`account-${current.action}-${current.account.id}`)?.focus();
  }
  function confirmKey(event: KeyboardEvent) {
    if (event.key === 'Escape') { event.preventDefault(); void cancelPending(); }
  }
  const finish = (event: SubmitEvent) => {
    event.preventDefault();
    if (!login || !callback.trim()) return;
    const id = login.id;
    const callbackURL = callback.trim();
    return run(async () => {
      const account = await api.completeAccountLogin(id, callbackURL, name.trim());
      login = undefined; callback = ''; name = '';
      message = `${account.name} wurde gespeichert. Zum Wechseln „Aktivieren“ wählen.`;
      await load();
    });
  };
  async function copyURL() {
    if (!login) return;
    try { await navigator.clipboard.writeText(login.url); copied = true; }
    catch { error = 'Bitte die Login-URL im Feld markieren und manuell kopieren.'; }
  }
  const confirm = () => run(async () => {
    if (!pending) return;
    const { account, action } = pending;
    pending = undefined;
    try {
      if (action === 'activate') {
        waitingSwitch = true; switchTarget = account.name; switchInfo = undefined;
        let result;
        try { result = await api.activateAccount(account.id); } finally { waitingSwitch = false; }
        testResult = result.test;
        message = `${result.account.name} ist jetzt aktiv.`;
        onchange();
      } else { await api.removeAccount(account.id); message = 'Account entfernt.'; }
    } finally { await load(); }
  });
  const test = (account: Account) => run(async () => { testResult = await api.testAccount(account.id); onchange(); await load(); });
  async function upload(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    await run(async () => {
      if (file.size > 1024 * 1024) throw new Error('Token-Datei ist zu groß (maximal 1 MB).');
      let token: unknown;
      try { token = JSON.parse(await file.text()); } catch { throw new Error('Die Token-Datei enthält kein gültiges JSON.'); }
      const account = await api.importAccount(name.trim(), token);
      name = ''; message = `${account.name} wurde gespeichert.`; await load();
    });
    input.value = '';
  }
</script>

<section aria-label="Account-Verwaltung" aria-busy={busy}>
  <div class="actions">
    <button class="btn primary" disabled={busy} onclick={() => start()}>Account hinzufügen</button>
    <label class="btn upload" class:disabled={busy}>
      Token-Datei importieren
      <input type="file" accept=".json,application/json" aria-label="Token-Datei importieren" disabled={busy} onchange={upload} />
    </label>
    <button class="btn" disabled={busy} onclick={() => run(load)}>Liste aktualisieren</button>
  </div>
  {#if error}<p class="err" role="alert">{error}</p>{/if}
  {#if message}<p class="muted" role="status">{message}</p>{/if}
  {#if busy}
    <p class="muted" role="status">
      {#if waitingSwitch}
        {#if switchInfo?.switching && switchInfo.running_requests > 0}Warte auf {switchInfo.running_requests} laufende {switchInfo.running_requests === 1 ? 'Anfrage' : 'Anfragen'}. Danach wird {switchTarget} automatisch aktiviert.
        {:else if switchInfo?.waiting}Warte auf den laufenden Test oder Wechsel. Danach wird {switchTarget} automatisch aktiviert.
        {:else if switchInfo?.switching}{switchTarget} wird aktiviert und getestet …
        {:else}Wechsel zu {switchTarget} vorgemerkt. Laufende Anfragen werden zuerst abgeschlossen …{/if}
      {:else}Bitte warten …{/if}
    </p>
    {#if waitingSwitch && switchInfo?.waiting && switchInfo.running_requests > 0}
      <p class="muted"><button class="btn" disabled={forcing} onclick={forceSwitch}>Sofort wechseln</button> bricht die laufenden Anfragen ab; Claude Code wiederholt sie.</p>
    {/if}
  {/if}
  {#if testResult}
    <p class:err={!testResult.ok} class="test-result" role="status">
      <StatusDot state={testResult.ok ? 'ok' : 'warn'} label={testResult.ok ? 'Test erfolgreich' : 'Test fehlgeschlagen'} />
      {testResult.message} <span class="muted">{testResult.model}{testResult.status ? ` · HTTP ${testResult.status}` : ''}</span>
    </p>
  {/if}
  {#if login}
    <form class="login panel" onsubmit={finish}>
      <h3>Account hinzufügen</h3>
      <label>Name (optional)<input bind:value={name} use:focusOnMount maxlength="120" autocomplete="off" placeholder="Zweites Abo" disabled={busy} /></label>
      <p>Öffne den Login-Link und melde dich mit dem gewünschten Claude-Account an. Falls der erste Account bereits angemeldet ist, nutze ein privates Browserfenster oder melde dich dort ab.</p>
      <a class="btn primary" href={login.url} target="_blank" rel="noopener noreferrer">Bei Claude anmelden</a>
      <label>Login-URL<textarea aria-label="Login-URL" readonly value={login.url} rows="3" spellcheck="false"></textarea></label>
      <button class="btn" type="button" onclick={copyURL}>{copied ? 'Kopiert' : 'Login-URL kopieren'}</button>
      <p>Nach der Freigabe öffnet Claude eine localhost-Adresse. Auch wenn diese Seite nicht erreichbar ist: Kopiere die vollständige URL aus der Adresszeile und füge sie hier ein. Funktioniert auch auf dem iPhone.</p>
      <label>Callback-URL<input aria-label="Callback-URL" bind:value={callback} type="url" autocomplete="off" spellcheck="false" placeholder="http://localhost:54545/callback?code=…&state=…" required disabled={busy} /></label>
      <p class="muted small">Login gültig bis {formatDateTime(login.expires_at)}. Neue Accounts werden zunächst inaktiv gespeichert.</p>
      <div class="actions"><button class="btn primary" type="submit" disabled={busy || !callback.trim()}>Account speichern</button><button class="btn" type="button" disabled={busy} onclick={() => { login = undefined; callback = ''; }}>Abbrechen</button></div>
    </form>
  {/if}
  {#if loaded && accounts.length === 0}<p class="muted">Noch kein Account gespeichert. Füge dein erstes Abo hinzu und aktiviere es.</p>{/if}
  <div class="account-list">
    {#each accounts as account (account.id)}
      <AccountCard {account} loading={usageLoading[account.id] ?? false}>
        {#if pending?.account.id === account.id}
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div class="confirm" onkeydown={confirmKey}>
            <p>{pending.action === 'activate' ? 'Prompt-Cache startet beim Wechsel der Organisation neu. Laufende Anfragen werden automatisch abgewartet; neue Anfragen warten bis zum Abschluss des Wechsels. Der Beispielaufruf verbraucht wenige Tokens.' : 'Diesen inaktiven Account aus dem Tresor entfernen?'}</p>
            <div class="actions"><button class="btn primary" use:focusOnMount disabled={busy} onclick={confirm}>{pending.action === 'activate' ? 'Wechseln und testen' : 'Entfernen bestätigen'}</button><button class="btn" disabled={busy} onclick={cancelPending}>Abbrechen</button></div>
          </div>
        {:else}
          <div class="actions">
            {#if account.active}<button class="btn" disabled={busy} onclick={() => test(account)}>Beispielaufruf testen</button>
            {:else}<button class="btn" id="account-activate-{account.id}" disabled={busy} onclick={() => { pending = { account, action: 'activate' }; }}>Aktivieren</button>{#if account.usage?.license?.token === TokenRelogin}<button class="btn" disabled={busy} onclick={() => start(account.name)}>Neu anmelden</button>{/if}<button class="btn" id="account-remove-{account.id}" disabled={busy} onclick={() => { pending = { account, action: 'remove' }; }}>Entfernen</button>{/if}
          </div>
        {/if}
      </AccountCard>
    {/each}
  </div>
</section>

<style>
  .actions { display: flex; flex-wrap: wrap; gap: 0.5rem; align-items: center; }
  .account-list { display: grid; gap: 0.9rem; margin-top: 1rem; }
  .panel { padding: 1.1rem; border-radius: var(--radius); background: var(--c-surface, var(--c-bg)); box-shadow: 0 0 0 1px var(--c-line); }
  h3 { font-size: 1rem; margin: 0; }
  .small { font-size: 0.85rem; }
  .login { display: grid; gap: 0.7rem; margin-top: 1rem; }
  .login p { margin: 0; line-height: 1.5; }
  .login label { display: grid; gap: 0.4rem; }
  input:not([type=file]), textarea { width: 100%; min-width: 0; box-sizing: border-box; font: inherit; padding: 0.6rem 0.7rem; color: var(--c-text); background: var(--c-bg); border: 1px solid var(--c-line); border-radius: var(--radius); }
  input:focus-visible, textarea:focus-visible { outline: 2px solid var(--c-accent); outline-offset: 2px; }
  textarea { font: inherit; font-size: 0.8rem; resize: vertical; overflow-wrap: anywhere; }
  a.btn { justify-self: start; text-decoration: none; }
  .upload { cursor: pointer; position: relative; }
  .upload input { position: absolute; inset: 0; opacity: 0; width: 100%; cursor: pointer; }
  .upload:focus-within { outline: 2px solid var(--c-accent); outline-offset: 3px; }
  .disabled { opacity: 0.5; cursor: default; }
  .test-result { display: flex; align-items: center; flex-wrap: wrap; gap: 0.5rem; line-height: 1.5; }
  .confirm p { line-height: 1.5; }
</style>
