// Local agent preferences in the Share-flow agent picker: pin to the top, hide
// into a collapsible section, and rename with a local alias. These are
// machine-local labels shared with the CLI; the UI just drives the backend.
import { describe, it, expect } from 'vitest';
import { mount, click } from './harness';

const staged = { PendingPaths: async () => ['/tmp/report.pdf'] };

type AgentOver = Partial<{
  agentId: string; name: string; alias: string; pinned: boolean; hidden: boolean; status: string;
}>;
const agent = (o: AgentOver) => ({
  agentId: '', sessionId: 's', deviceId: 'd', deviceName: 'box', tool: 'claude',
  name: 'claude', status: 'online', lastSeen: '', alias: '', pinned: false, hidden: false, ...o,
});

/** Mount with a staged file and open the "An agent" destination. */
async function picker(agents: unknown[]) {
  const m = await mount({ ...staged, ListAgents: async () => agents });
  await click(m, '[data-dest-opt="agent"]');
  await m.settle(5); // the agent list loads when the destination opens
  return m;
}

describe('the agent picker local prefs', () => {
  it('shows the alias in place of the server name, and pin/rename/hide actions', async () => {
    const m = await picker([agent({ agentId: 'agt_aaaaaaaaaaaaaaaa', name: 'claude-1', alias: 'Reviewer' })]);
    expect(m.text()).toContain('Reviewer');
    expect(m.text()).not.toContain('claude-1');
    expect(m.$('.agent-pin')).not.toBeNull();
    expect(m.$('.agent-rename')).not.toBeNull();
    expect(m.$('.agent-hide')).not.toBeNull();
  });

  it('sorts pinned agents first and tucks hidden ones into a section', async () => {
    const m = await picker([
      agent({ agentId: 'agt_aaaaaaaaaaaaaaaa', name: 'alpha' }),
      agent({ agentId: 'agt_bbbbbbbbbbbbbbbb', name: 'bravo', pinned: true }),
      agent({ agentId: 'agt_cccccccccccccccc', name: 'charlie', hidden: true }),
    ]);
    const rows = m.$$('.agent-pick-sel');
    expect(rows[0].textContent).toContain('bravo'); // pinned first
    expect(m.text()).toMatch(/Hidden agents \(1\)/i); // hidden one in its own section
  });

  it('pins through the backend with the toggled value', async () => {
    const m = await picker([agent({ agentId: 'agt_aaaaaaaaaaaaaaaa', name: 'alpha' })]);
    await click(m, '.agent-pin');
    expect(m.calls.PinAgent?.[0]).toEqual(['agt_aaaaaaaaaaaaaaaa', true]);
  });

  it('hides through the backend with the toggled value', async () => {
    const m = await picker([agent({ agentId: 'agt_aaaaaaaaaaaaaaaa', name: 'alpha' })]);
    await click(m, '.agent-hide');
    expect(m.calls.HideAgent?.[0]).toEqual(['agt_aaaaaaaaaaaaaaaa', true]);
  });

  it('renames via an overlay and saves the local alias', async () => {
    const m = await picker([agent({ agentId: 'agt_aaaaaaaaaaaaaaaa', name: 'alpha' })]);
    await click(m, '.agent-rename');
    const input = m.$('#agent-rename-input') as HTMLInputElement | null;
    expect(input).not.toBeNull();
    input!.value = 'Build bot';
    await click(m, '#agent-rename-save');
    expect(m.calls.RenameAgent?.[0]).toEqual(['agt_aaaaaaaaaaaaaaaa', 'Build bot']);
  });
});
