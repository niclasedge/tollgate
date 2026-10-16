import { afterEach, expect, it, vi } from 'vitest';
import { render, screen, fireEvent, waitFor, within } from '@testing-library/svelte';
import AccountManager from './lib/views/AccountManager.svelte';
import { auth, logout } from './lib/auth.svelte';
import { json, mockFetch } from './test-helpers';
import type { Account, UsageSnapshot } from './lib/types/accounts';
const a: Account = { id: 'a', name: 'Erstes Abo', email: 'a@example.com', active: true, status: 'active', expires_at: '', last_refresh: '', updated_at: '' };
const b: Account = { ...a, id: 'b', name: 'Zweites Abo', email: 'b@example.com', active: false, status: 'gespeichert' };
afterEach(() => { logout(); vi.unstubAllGlobals(); });
it('shows login URL and completes callback without switching the current account', async () => {
  auth.token = 'admin'; let list = [a];
  const f = mockFetch({
    'GET /api/accounts': () => json({ accounts: list }),
    'POST /api/accounts/login': () => json({ id: 'login-state', url: 'https://claude.ai/oauth/authorize?state=login-state', expires_at: '2026-10-04T23:00:00Z' }),
    'POST /api/accounts/login/complete': (init) => { expect(JSON.parse(String(init?.body))).toMatchObject({ id: 'login-state', callback_url: 'http://localhost:54545/callback?code=test&state=login-state' }); list = [a,b]; return json(b,201); },
  });
  render(AccountManager);
  await screen.findByText('a@example.com');
  await fireEvent.click(screen.getByRole('button', { name: 'Account hinzufügen' }));
  expect(await screen.findByRole('link', { name: 'Bei Claude anmelden' })).toHaveAttribute('href','https://claude.ai/oauth/authorize?state=login-state');
  expect(screen.getByLabelText('Login-URL')).toHaveValue('https://claude.ai/oauth/authorize?state=login-state');
  await fireEvent.input(screen.getByLabelText('Callback-URL'), { target: { value: 'http://localhost:54545/callback?code=test&state=login-state' } });
  await fireEvent.click(screen.getByRole('button', { name: 'Account speichern' }));
  expect(await screen.findByText('b@example.com')).toBeInTheDocument();
  expect(f.mock.calls.some(([path]) => String(path).includes('/activate'))).toBe(false);
  expect(screen.queryByLabelText('Callback-URL')).not.toBeInTheDocument();
});
it('requires inline confirmation and shows the test result after activation', async () => {
 auth.token='admin';let list=[a,b];
 const f=mockFetch({ 'GET /api/accounts':()=>json({accounts:list}), 'POST /api/accounts/b/activate':()=>{list=[{...a,active:false},{...b,active:true}];return json({account:list[1],cache_notice:'Prompt-Cache startet neu',test:{ok:true,status:200,model:'claude-sonnet-5-5',message:'Beispielaufruf erfolgreich'}})} });
 const changed=vi.fn();render(AccountManager,{onchange:changed});
 await screen.findByText('b@example.com');await fireEvent.click(screen.getByRole('button',{name:'Aktivieren'}));
 expect(screen.getByText(/Prompt-Cache startet/)).toBeInTheDocument();expect(f.mock.calls.some(([p])=>String(p).includes('/activate'))).toBe(false);
 await fireEvent.click(screen.getByRole('button',{name:'Wechseln und testen'}));
 expect(await screen.findByText(/Beispielaufruf erfolgreich/)).toBeInTheDocument();await waitFor(()=>expect(changed).toHaveBeenCalled());
 expect(screen.getByRole('button',{name:'Beispielaufruf testen'})).toBeInTheDocument();
});
it('shows rollback failure and keeps the old account active', async()=>{
 auth.token='admin';mockFetch({'GET /api/accounts':()=>json({accounts:[a,b]}),'POST /api/accounts/b/activate':()=>json({error:'Wechsel fehlgeschlagen; vorheriger Zustand wurde wiederhergestellt'},409)});
 render(AccountManager);await screen.findByText('b@example.com');await fireEvent.click(screen.getByRole('button',{name:'Aktivieren'}));await fireEvent.click(screen.getByRole('button',{name:'Wechseln und testen'}));
 expect(await screen.findByRole('alert')).toHaveTextContent('wiederhergestellt');expect(screen.getByRole('button',{name:'Beispielaufruf testen'})).toBeInTheDocument();
});

it('loads account usage without triggering a reactive request loop',async()=>{
 auth.token='admin';
 const snapshot={five_hour:{utilization:10,reset:new Date(Date.now()+3600000).toISOString()},seven_day:{utilization:80,reset:new Date(Date.now()+3*86400000).toISOString()},observed_at:new Date().toISOString(),error:''};
 const f=mockFetch({'GET /api/accounts':()=>json({accounts:[a,b]}),'GET /api/accounts/a/usage':()=>json(snapshot),'GET /api/accounts/b/usage':()=>json({...snapshot,seven_day:{...snapshot.seven_day,utilization:100}})});
 render(AccountManager);await screen.findByText('20 % übrig');await screen.findByText('Wochenlimit erreicht');
 await new Promise(resolve=>setTimeout(resolve,50));
 expect(f.mock.calls.filter(([path])=>path==='/api/accounts')).toHaveLength(1);
 expect(f.mock.calls.filter(([path])=>String(path).endsWith('/usage'))).toHaveLength(2);
});

it('shows queued switch progress and completes without another click',async()=>{
 auth.token='admin';let finish!:(r:Response)=>void;let list=[a,b];
 mockFetch({'GET /api/accounts':()=>json({accounts:list}),'GET /api/accounts/switch-status':()=>json({switching:true,running_requests:2}),'POST /api/accounts/b/activate':()=>new Promise<Response>(resolve=>{finish=resolve})});
 render(AccountManager);await screen.findByText('b@example.com');await fireEvent.click(screen.getByRole('button',{name:'Aktivieren'}));await fireEvent.click(screen.getByRole('button',{name:'Wechseln und testen'}));
 expect(await screen.findByText(/Warte auf 2 laufende Anfragen/)).toBeInTheDocument();
 list=[{...a,active:false},{...b,active:true}];finish(json({account:list[1],test:{ok:true,status:200,model:'claude-sonnet-5-5',message:'Beispielaufruf erfolgreich'},cache_notice:''}));
 expect(await screen.findByText('Zweites Abo ist jetzt aktiv.')).toBeInTheDocument();expect(screen.queryByText(/Warte auf 2/)).not.toBeInTheDocument();
});

const soon=(h:number)=>new Date(Date.now()+h*3600000).toISOString();
const usageWith=(license:Partial<NonNullable<UsageSnapshot['license']>>)=>({five_hour:{utilization:25,reset:soon(2)},seven_day:{utilization:40,reset:soon(72)},observed_at:new Date().toISOString(),error:'',license:{token:'valid',checked_at:'2026-10-04T10:00:00Z',organization_type:'',rate_limit_tier:'',subscription_status:'',profile_observed_at:'2026-10-04T10:00:00Z',...license}});
const licensed={
 'GET /api/accounts/a/usage':()=>json(usageWith({organization_type:'claude_max',rate_limit_tier:'default_claude_max_20x',subscription_status:'active'})),
 'GET /api/accounts/b/usage':()=>json(usageWith({token:'relogin',organization_type:'claude_pro'})),
};
it('shows token state, plan and a dated license for every subscription',async()=>{
 auth.token='admin';mockFetch({'GET /api/accounts':()=>json({accounts:[a,b]}),...licensed});
 render(AccountManager);
 const first=await screen.findByRole('article',{name:'a@example.com'});const second=screen.getByRole('article',{name:'b@example.com'});
 expect(await within(first).findByText('Token gültig')).toBeInTheDocument();expect(within(first).getByText(/Max 20x · Abo aktiv/)).toBeInTheDocument();
 expect(await within(second).findByText('Neu einloggen nötig')).toBeInTheDocument();expect(within(second).getByText(/^Pro/)).toBeInTheDocument();
 expect(within(second).getByText(/Lizenz: Stand vom/)).toBeInTheDocument();expect(within(first).queryByText(/Lizenz: Stand vom/)).not.toBeInTheDocument();
 expect(within(first).getAllByRole('meter')).toHaveLength(2);expect(within(second).getAllByRole('meter')).toHaveLength(2);
});
it('offers a new login for an inactive subscription that needs it',async()=>{
 auth.token='admin';const f=mockFetch({'GET /api/accounts':()=>json({accounts:[a,b]}),...licensed,'POST /api/accounts/login':()=>json({id:'login-state',url:'https://claude.ai/oauth/authorize?state=login-state',expires_at:'2026-10-04T23:00:00Z'})});
 render(AccountManager);const second=await screen.findByRole('article',{name:'b@example.com'});
 await fireEvent.click(await within(second).findByRole('button',{name:'Neu anmelden'}));
 expect(await screen.findByLabelText(/Name/)).toHaveValue('Zweites Abo');
 expect(f.mock.calls.some(([p])=>String(p).includes('/activate'))).toBe(false);
});
it('cancels the inline confirmation with Escape and returns focus',async()=>{
 auth.token='admin';const f=mockFetch({'GET /api/accounts':()=>json({accounts:[a,b]})});
 render(AccountManager);await screen.findByText('b@example.com');
 await fireEvent.click(screen.getByRole('button',{name:'Aktivieren'}));
 const confirmButton=screen.getByRole('button',{name:'Wechseln und testen'});expect(confirmButton).toHaveFocus();
 await fireEvent.keyDown(confirmButton,{key:'Escape'});
 expect(screen.queryByRole('button',{name:'Wechseln und testen'})).not.toBeInTheDocument();
 await waitFor(()=>expect(screen.getByRole('button',{name:'Aktivieren'})).toHaveFocus());
 expect(f.mock.calls.some(([p])=>String(p).includes('/activate'))).toBe(false);
});
it('acceptance: switches from subscription 1 to 2 and shows both licenses',async()=>{
 auth.token='admin';let list=[a,b];
 mockFetch({'GET /api/accounts':()=>json({accounts:list}),...licensed,'GET /api/accounts/switch-status':()=>json({waiting:false,switching:true,running_requests:0}),
  'POST /api/accounts/b/activate':()=>{list=[{...a,active:false,status:'gespeichert'},{...b,active:true,status:'active'}];return json({account:list[1],cache_notice:'Prompt-Cache startet neu',test:{ok:true,status:200,model:'claude-sonnet-5-5',message:'Beispielaufruf erfolgreich'}});}});
 render(AccountManager);const second=await screen.findByRole('article',{name:'b@example.com'});
 await fireEvent.click(within(second).getByRole('button',{name:'Aktivieren'}));
 expect(within(second).getByText(/Prompt-Cache startet/)).toBeInTheDocument();
 await fireEvent.click(within(second).getByRole('button',{name:'Wechseln und testen'}));
 expect(await screen.findByText('Zweites Abo ist jetzt aktiv.')).toBeInTheDocument();
 await waitFor(()=>expect(within(screen.getByRole('article',{name:'b@example.com'})).getByRole('img',{name:'Aktiv'})).toBeInTheDocument());
 expect(within(screen.getByRole('article',{name:'a@example.com'})).getByRole('img',{name:'Inaktiv'})).toBeInTheDocument();
 expect(within(screen.getByRole('article',{name:'a@example.com'})).getByText('Token gültig')).toBeInTheDocument();
 expect(within(screen.getByRole('article',{name:'b@example.com'})).getByText('Neu einloggen nötig')).toBeInTheDocument();
 expect(within(screen.getByRole('article',{name:'a@example.com'})).getByRole('button',{name:'Aktivieren'})).toBeInTheDocument();
});
it('forces a queued switch while requests are running',async()=>{
 auth.token='admin';let finish!:(r:Response)=>void;let list=[a,b];
 const f=mockFetch({'GET /api/accounts':()=>json({accounts:list}),'GET /api/accounts/switch-status':()=>json({waiting:true,switching:true,running_requests:1}),
  'POST /api/accounts/switch/force':()=>json({waiting:true,switching:true,running_requests:1}),'POST /api/accounts/b/activate':()=>new Promise<Response>(resolve=>{finish=resolve})});
 render(AccountManager);await screen.findByText('b@example.com');await fireEvent.click(screen.getByRole('button',{name:'Aktivieren'}));await fireEvent.click(screen.getByRole('button',{name:'Wechseln und testen'}));
 await fireEvent.click(await screen.findByRole('button',{name:'Sofort wechseln'}));
 expect(f.mock.calls.some(([p,init])=>p==='/api/accounts/switch/force'&&init?.method==='POST')).toBe(true);
 list=[{...a,active:false},{...b,active:true}];finish(json({account:list[1],test:{ok:true,status:200,model:'claude-sonnet-5-5',message:'Beispielaufruf erfolgreich'},cache_notice:''}));
 expect(await screen.findByText('Zweites Abo ist jetzt aktiv.')).toBeInTheDocument();expect(screen.queryByRole('button',{name:'Sofort wechseln'})).not.toBeInTheDocument();
});
