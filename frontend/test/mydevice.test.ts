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

// "Is my laptop set up to receive?" is a question people ask BEFORE picking a
// file. The send modal could answer it, but only once you were mid-send —
// starting a share you may not want just to read a status.
describe('the devices list in Settings', () => {
  it('lists the account\'s devices with the same states as the send flow', async () => {
    const m = await mount({ ListDevices: async () => [laptop, keyless, thisMachine] });
    await click(m, '#open-settings');

    expect(m.text()).toContain('Your devices');
    expect(m.text()).toContain('laptop');
    expect(m.text()).toMatch(/ready to receive/i);
    expect(m.text()).toMatch(/can't receive yet/i);
    // Unlike the send flow this DOES show the current machine: the question here
    // is what the account looks like, not where a file can go.
    expect(m.text()).toContain('openclaw');
  });

  // A network call most sessions never need should not run at startup.
  it('does not fetch until Settings is opened', async () => {
    const m = await mount({ ListDevices: async () => [laptop] });
    expect(m.calls.ListDevices).toBeUndefined();
    await click(m, '#open-settings');
    expect(m.calls.ListDevices?.length).toBe(1);
  });

  it('can be refreshed', async () => {
    const m = await mount({ ListDevices: async () => [laptop] });
    await click(m, '#open-settings');
    await click(m, '#devices-refresh');
    expect(m.calls.ListDevices?.length).toBe(2);
  });

  it('says so when there are no other devices, rather than showing nothing', async () => {
    const m = await mount({ ListDevices: async () => [] });
    await click(m, '#open-settings');
    expect(m.text()).toMatch(/Only this one so far/i);
  });

  it('is not shown when signed out', async () => {
    const m = await mount({
      ListDevices: async () => [laptop],
      Status: async () => ({
        loggedIn: false, email: '', isApiToken: false, canReceive: false,
        shellInstalled: false, autostartEnabled: false, discoverable: true,
      }),
    });
    await click(m, '#open-settings');
    expect(m.text()).not.toContain('Your devices');
    expect(m.calls.ListDevices).toBeUndefined();
  });
});

// Settings used to hold its open state only in the DOM, and render() rebuilds
// that element -- so ANY re-render while Settings was open shut it, including
// the ones its own controls trigger. Loading the device list is one such
// re-render, which is how this surfaced.
describe('Settings survives a re-render', () => {
  it('stays open while the device list loads', async () => {
    const m = await mount({ ListDevices: async () => [laptop] });
    await click(m, '#open-settings');
    await m.settle(20); // let the fetch resolve and re-render
    expect((m.$('details.settings') as HTMLDetailsElement).open).toBe(true);
    expect(m.text()).toContain('laptop');
  });

  it('stays open when one of its own controls re-renders the page', async () => {
    const m = await mount({ ListDevices: async () => [laptop] });
    await click(m, '#open-settings');
    await click(m, '#set-discoverable');
    await m.settle(20);
    expect((m.$('details.settings') as HTMLDetailsElement).open).toBe(true);
  });
});

// Anything that outlives BUSY_DELAY has to say so -- that is the app's
// convention, and a Refresh button that makes a network call and shows nothing
// looks broken rather than busy.
describe('the device controls report that they are working', () => {
  it('marks Refresh busy while the list loads, and clears it after', async () => {
    let release: (v: unknown) => void = () => {};
    const held = new Promise((r) => { release = r; });
    const m = await mount({ ListDevices: async () => { await held; return [laptop]; } });
    await click(m, '#open-settings');
    await click(m, '#devices-refresh');
    await m.settle(180); // past BUSY_DELAY (120ms)

    expect(m.$('#devices-refresh')?.classList.contains('is-busy')).toBe(true);
    release([laptop]);
    await m.settle(30);
    expect(m.$('#devices-refresh')?.classList.contains('is-busy')).toBe(false);
  });
});
