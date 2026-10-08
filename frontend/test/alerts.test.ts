// Notification alerts: a sound + desktop popup on new files / agent requests,
// gated by a Settings toggle. Here we check the two things unit-testable without
// a real audio device: startup suppression, and the toggle wiring.
import { describe, it, expect } from 'vitest';
import { mount } from './harness';

const oneRequest = {
  ListPendingRequests: async () => [
    { id: 'req-1', senderDeviceId: 'd1', senderName: 'Laptop', tool: 'claude', hasFile: true, createdAt: '' },
  ],
};

describe('notification alerts', () => {
  it('does not alert for agent requests that already exist at startup', async () => {
    const m = await mount({ ...oneRequest });
    await m.settle(10);
    // First load adopts the current pending set silently — no sound on launch.
    expect(m.calls.Alert).toBeUndefined();
  });

  it('Settings exposes a sound+popup toggle wired to SetAlerts', async () => {
    const m = await mount({
      Status: async () => ({
        loggedIn: true, email: 'me@example.com', isApiToken: false, canReceive: true,
        shellInstalled: false, autostartEnabled: false, discoverable: true, alerts: true,
      }),
    });
    await m.settle(5);
    const box = m.$('#set-alerts') as HTMLInputElement | null;
    expect(box).not.toBeNull();
    expect(box!.checked).toBe(true); // reflects status.alerts
    box!.checked = false;
    box!.dispatchEvent(new Event('change', { bubbles: true }));
    await m.settle(5);
    expect(m.calls.SetAlerts?.[0]).toEqual([false]);
  });
});
