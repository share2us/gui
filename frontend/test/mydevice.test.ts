// Sending to your own device OVER THE INTERNET, not just across the LAN.
//
// The Go side could always do this (App.Share target "device", App.ListDevices),
// but nothing in the UI reached it: the only device sends the app offered were
// LAN ones, so the feature was invisible outside the CLI (§AG Gap 1).
import { describe, it, expect } from 'vitest';
import { mount, click } from './harness';

const laptop = { sessionId: 'sess-1', name: 'laptop', label: 'laptop:linux', publicKey: 'pk-1', hasKey: true, current: false };
const keyless = { sessionId: 'sess-2', name: 'phone', label: 'phone:android', publicKey: '', hasKey: false, current: false };
const thisMachine = { sessionId: 'sess-3', name: 'openclaw', label: 'openclaw:linux', publicKey: 'pk-3', hasKey: true, current: true };

const staged = { PendingPaths: async () => ['/tmp/report.pdf'] };

/** Open the share modal with a file staged, on the device tab. */
async function deviceTab(overrides: Record<string, unknown> = {}) {
  return mount({ ...staged, ...overrides });
}

describe('the "My device, anywhere" destination', () => {
  it('is offered beside the LAN options', async () => {
    const m = await deviceTab({ ListDevices: async () => [laptop] });
    expect(m.$('[data-dest-opt="nearby"]')).not.toBeNull();
    expect(m.$('[data-dest-opt="mydevice"]')).not.toBeNull();
    expect(m.$('[data-dest-opt="broadcast"]')).not.toBeNull();
  });

  // A network call most sends never need should not run on startup.
  it('does not fetch the device list until the option is opened', async () => {
    const m = await deviceTab({ ListDevices: async () => [laptop] });
    expect(m.calls.ListDevices).toBeUndefined();
    await click(m, '[data-dest-opt="mydevice"]');
    expect(m.calls.ListDevices?.length).toBe(1);
  });

  it('says the transfer is not limited to this network', async () => {
    const m = await deviceTab({ ListDevices: async () => [laptop] });
    await click(m, '[data-dest-opt="mydevice"]');
    expect(m.text()).toMatch(/works from anywhere/i);
  });
});

describe('which devices it offers', () => {
  it('lists the account\'s other devices', async () => {
    const m = await deviceTab({ ListDevices: async () => [laptop] });
    await click(m, '[data-dest-opt="mydevice"]');
    expect(m.text()).toContain('laptop');
  });

  // Sending a file to the machine you are sitting at is never what you meant.
  it('leaves out the machine you are on', async () => {
    const m = await deviceTab({ ListDevices: async () => [laptop, thisMachine] });
    await click(m, '[data-dest-opt="mydevice"]');
    expect(m.text()).not.toContain('openclaw');
  });

  // "My laptop is missing" is a worse puzzle than a dimmed row explaining why.
  it('shows a device with no encryption key, and says what to do', async () => {
    const m = await deviceTab({ ListDevices: async () => [keyless] });
    await click(m, '[data-dest-opt="mydevice"]');
    expect(m.text()).toContain('phone');
    expect(m.text()).toMatch(/sign in with the app on that device/i);
    const btn = m.$('.pick-cloud') as HTMLButtonElement;
    expect(btn.disabled).toBe(true);
  });

  it('explains an empty list rather than showing nothing', async () => {
    const m = await deviceTab({ ListDevices: async () => [] });
    await click(m, '[data-dest-opt="mydevice"]');
    expect(m.text()).toMatch(/No other devices are signed in/i);
  });

  it('offers a retry when the list cannot be loaded', async () => {
    const m = await deviceTab({ ListDevices: async () => { throw new Error('offline'); } });
    await click(m, '[data-dest-opt="mydevice"]');
    expect(m.$('#cloud-retry')).not.toBeNull();
    await click(m, '#cloud-retry');
    expect(m.calls.ListDevices?.length).toBe(2);
  });
});

describe('sending', () => {
  it('will not send until a device is chosen', async () => {
    const m = await deviceTab({ ListDevices: async () => [laptop] });
    await click(m, '[data-dest-opt="mydevice"]');
    expect((m.$('#primary-btn') as HTMLButtonElement).disabled).toBe(true);
    expect(m.text()).toMatch(/Pick one of your devices/i);
  });

  it('names the chosen device on the button', async () => {
    const m = await deviceTab({ ListDevices: async () => [laptop] });
    await click(m, '[data-dest-opt="mydevice"]');
    await click(m, '.pick-cloud');
    expect((m.$('#primary-btn') as HTMLButtonElement).textContent).toContain('laptop');
  });

  it('sends sealed to that device, not as a link', async () => {
    const m = await deviceTab({
      ListDevices: async () => [laptop],
      Share: async () => [{ path: '/tmp/report.pdf', ok: true }],
    });
    await click(m, '[data-dest-opt="mydevice"]');
    await click(m, '.pick-cloud');
    await click(m, '#primary-btn');

    const sent = (m.calls.Share ?? [])[0]?.[0] as { target: string; deviceId: string; devicePub: string } | undefined;
    expect(sent).toBeDefined();
    expect(sent?.target).toBe('device');
    expect(sent?.deviceId).toBe('sess-1');
    expect(sent?.devicePub).toBe('pk-1');
  });
});

describe('a login is required', () => {
  it('blocks the send and does not try to list devices when signed out', async () => {
    const m = await deviceTab({
      ListDevices: async () => [laptop],
      Status: async () => ({
        loggedIn: false, email: '', isApiToken: false, canReceive: false,
        shellInstalled: false, autostartEnabled: false, discoverable: true,
      }),
    });
    await click(m, '[data-dest-opt="mydevice"]');
    expect(m.calls.ListDevices).toBeUndefined();
    expect((m.$('#primary-btn') as HTMLButtonElement).disabled).toBe(true);
  });
});
