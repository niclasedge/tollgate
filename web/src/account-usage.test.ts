import { expect, it, vi, afterEach } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import AccountUsage from './lib/components/AccountUsage.svelte';
afterEach(()=>vi.useRealTimers());
it('compares weekly consumption with elapsed time and reset',()=>{
 vi.useFakeTimers();vi.setSystemTime(new Date('2026-10-04T12:00:00Z'));
 render(AccountUsage,{usage:{five_hour:{utilization:10,reset:'2026-10-04T16:00:00Z'},seven_day:{utilization:80,reset:'2026-10-08T00:00:00Z'},observed_at:'2026-10-04T11:59:00Z',error:''}});
 expect(screen.getByText('20 % übrig')).toBeInTheDocument();
 expect(screen.getByRole('meter',{name:'Wochenverbrauch'})).toHaveAttribute('aria-valuenow','80');
 expect(screen.getByLabelText('50 % der Woche vergangen')).toBeInTheDocument();
 expect(screen.getByText(/Reset in 3 T 12 h/)).toBeInTheDocument();
});
it('does not claim availability after reset or without data',()=>{
 vi.useFakeTimers();vi.setSystemTime(new Date('2026-10-04T12:00:00Z'));
 const view=render(AccountUsage,{usage:{five_hour:{utilization:null,reset:''},seven_day:{utilization:100,reset:'2026-10-04T11:00:00Z'},observed_at:'2026-10-04T10:00:00Z',error:''}});
 expect(screen.getByText('Reset erfolgt · neuer Messstand nötig')).toBeInTheDocument();expect(screen.queryByText(/übrig/)).not.toBeInTheDocument();
 view.unmount();render(AccountUsage);expect(screen.getByText('Wochenstatus unbekannt')).toBeInTheDocument();
});
it('marks exhausted budget and stale failed reads',()=>{
 vi.useFakeTimers();vi.setSystemTime(new Date('2026-10-04T12:00:00Z'));
 render(AccountUsage,{usage:{five_hour:{utilization:100,reset:'2026-10-04T16:00:00Z'},seven_day:{utilization:100,reset:'2026-10-08T00:00:00Z'},observed_at:'2026-10-04T10:00:00Z',error:'Limits momentan nicht abrufbar'}});
 expect(screen.getByText('Wochenlimit erreicht')).toBeInTheDocument();expect(screen.getByText(/Veralteter Messstand/)).toBeInTheDocument();expect(screen.getByText('Limits momentan nicht abrufbar')).toBeInTheDocument();
});

it('does not infer free budget from a percentage without a valid window',()=>{
 vi.useFakeTimers();vi.setSystemTime(new Date('2026-10-04T12:00:00Z'));
 render(AccountUsage,{usage:{five_hour:{utilization:null,reset:''},seven_day:{utilization:0,reset:''},observed_at:'2026-10-04T11:59:00Z',error:''}});
 expect(screen.getByText('Wochenstatus unbekannt')).toBeInTheDocument();expect(screen.queryByText(/übrig/)).not.toBeInTheDocument();
});

it('keeps a preserved weekly measurement stale when only the session was updated',()=>{
 vi.useFakeTimers();vi.setSystemTime(new Date('2026-10-04T12:00:00Z'));
 render(AccountUsage,{usage:{five_hour:{utilization:10,reset:'2026-10-04T16:00:00Z',observed_at:'2026-10-04T11:59:00Z'},seven_day:{utilization:80,reset:'2026-10-08T00:00:00Z',observed_at:'2026-10-04T10:00:00Z'},observed_at:'2026-10-04T11:59:00Z',error:''}});
 expect(screen.getByText(/Veralteter Messstand/)).toBeInTheDocument();expect(screen.getByText('Zuletzt 20 % übrig')).toBeInTheDocument();
});

it('shows the five-hour window as a subtle bar with reset and dates the measurement',()=>{
 vi.useFakeTimers();vi.setSystemTime(new Date('2026-10-04T12:00:00Z'));
 render(AccountUsage,{usage:{five_hour:{utilization:10,reset:'2026-10-04T16:00:00Z'},seven_day:{utilization:80,reset:'2026-10-08T00:00:00Z'},observed_at:'2026-10-04T11:59:00Z',error:''}});
 expect(screen.getByRole('meter',{name:'5-Stunden-Verbrauch'})).toHaveAttribute('aria-valuenow','10');
 expect(screen.getByText(/^Reset in 4 h/)).toBeInTheDocument();
 expect(screen.getByText(/^Stand vom /)).toBeInTheDocument();
});
