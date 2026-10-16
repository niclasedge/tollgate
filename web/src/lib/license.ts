// Display texts for the license state of one subscription.
import { TokenExpired, TokenRelogin, TokenValid, type License } from './types/accounts';

const plans: Record<string, string> = { claude_max: 'Max', claude_pro: 'Pro', claude_team: 'Team', claude_enterprise: 'Enterprise' };
const statuses: Record<string, string> = { active: 'Abo aktiv', past_due: 'Zahlung offen', canceled: 'gekündigt', cancelled: 'gekündigt', trialing: 'Testphase' };

export function planLabel(l: License | undefined): string {
  if (!l?.organization_type) return 'Plan unbekannt';
  const plan = plans[l.organization_type] ?? l.organization_type;
  const tier = /_(\d+x)$/.exec(l.rate_limit_tier)?.[1];
  return tier && plans[l.organization_type] ? `${plan} ${tier}` : plan;
}

export function subscriptionLabel(l: License | undefined): string {
  if (!l?.subscription_status) return '';
  return statuses[l.subscription_status] ?? l.subscription_status;
}

export function tokenState(l: License | undefined, active: boolean): { state: 'ok' | 'warn' | 'idle'; label: string } {
  switch (l?.token) {
    case TokenValid: return { state: 'ok', label: 'Token gültig' };
    case TokenRelogin: return { state: 'warn', label: 'Neu einloggen nötig' };
    case TokenExpired: return { state: 'idle', label: `Zugang abgelaufen · ${active ? 'wird von CLIProxyAPI erneuert' : 'wird beim Aktivieren erneuert'}` };
    default: return { state: 'idle', label: 'Token-Status unbekannt' };
  }
}
