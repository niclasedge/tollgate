import {afterEach,it,expect,vi} from 'vitest';
import {render,screen,fireEvent,within} from '@testing-library/svelte';
import Live from './lib/views/Live.svelte';
import Costs from './lib/views/Costs.svelte';
import {auth,logout} from './lib/auth.svelte';
import {trace,costRow,json,mockFetch,sseResponse} from './test-helpers';
afterEach(()=>{logout();vi.unstubAllGlobals()});
it('shows saved account ID in live details and leaves older requests unknown',async()=>{
 auth.token='admin';const stream=sseResponse();const id='a'.repeat(64);
 mockFetch({'GET /api/traces':()=>json({traces:[trace({account_id:id}),trace({id:'old',account_id:undefined})],next_before:'',next_before_id:''}),'GET /api/traces/events':()=>stream.response});
 render(Live);await screen.findByText('Account '+id.slice(0,12));
 await fireEvent.click(screen.getAllByRole('button')[0]);expect(screen.getByText(id)).toBeInTheDocument();
 expect(screen.getByText('Account unbekannt')).toBeInTheDocument();stream.end();
});
it('keeps per-account costs separate from unassigned historical usage',async()=>{
 auth.token='admin';mockFetch({'GET /api/performance':()=>json({from:'2026-10-04',to:'2026-10-04',models:[],redactor:null,speedtests:[],fastest_fill:null,capacity:{windows_per_week:null,fills_per_day:null,measured_hours: 0, sample_windows: 0, window_cost_nano_usd: null, sample_accounts: 0, accounts: 0, free_this_week: null }}),'GET /api/costs':()=>json({from:'2026-10-04',to:'2026-10-04',hours:[],rows:[costRow({account_id:'account-a',usage:{input:10,output:0,cache_read:0,cache_creation:0}}),costRow({account_id:'account-b',usage:{input:20,output:0,cache_read:0,cache_creation:0}}),costRow({usage:{input:30,output:0,cache_read:0,cache_creation:0}})]})});
 render(Costs);const table=within(await screen.findByRole('table',{name:'Aufschlüsselung'}));expect(table.getByText('Account account-a')).toBeInTheDocument();expect(table.getByText('Account account-b')).toBeInTheDocument();expect(table.getByText('Nicht zugeordnet')).toBeInTheDocument();
 expect(table.getByText('10')).toBeInTheDocument();expect(table.getByText('20')).toBeInTheDocument();expect(table.getByText('30')).toBeInTheDocument();
});
