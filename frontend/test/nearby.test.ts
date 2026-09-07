// Nearby devices: how a device is described, and what its controls do.
import { describe, it, expect } from 'vitest';
import { mount, click } from './harness';

const named = { name: 'kestrel', addr: '192.168.15.9:4300', dest: 's2u://192.168.15.9:4300?f=AA', code: '123 456', mode: 'open', fingerprint: 'AA', isBroadcast: false, fileName: '', fileSize: 0 };
const scanned = { ...named, name: '192.168.15.20', addr: '192.168.15.20:4300', fingerprint: 'BB', dest: 's2u://192.168.15.20:4300?f=BB', viaScan: true };

describe('a nearby device', () => {
  it('offers to send, and asks which files first', async () => {
    // From Home nothing is chosen yet. This used to call LanSend with an empty
    // list: nothing was sent, and an empty result counted as success, so nothing
    // was reported either.
    const m = await mount({ LanBrowse: async () => [named], PickFiles: async () => ['/tmp/a.pdf'] });
    await click(m, '.send-to');
    expect(m.calls.PickFiles?.length).toBe(1);
    expect(m.calls.LanSend).toBeUndefined();
  });

  it('names the chosen device on the button once files are picked', async () => {
    const m = await mount({ LanBrowse: async () => [named], PickFiles: async () => ['/tmp/a.pdf'] });
    await click(m, '.send-to');
    expect((m.$('#primary-btn') as HTMLButtonElement).textContent).toContain('kestrel');
  });

  it('sends nothing when the file picker is cancelled', async () => {
    const m = await mount({ LanBrowse: async () => [named], PickFiles: async () => [] });
    await click(m, '.send-to');
    expect(m.calls.LanSend).toBeUndefined();
  });

  it('shows a scan-found device by address rather than inventing a name', async () => {
    const m = await mount({ LanBrowse: async () => [scanned] });
    expect(m.text()).toContain('192.168.15.20');
    expect(m.text()).toMatch(/name not announced/i);
  });
});

describe('this device', () => {
  it('shows its own address beside its verify code', async () => {
    // So the person reading someone else's device list can tell which entry is
    // theirs, without going to look it up in the operating system.
    const m = await mount();
    m.emit('lan-discoverable', { address: '192.168.15.114:4300', code: '123 456', safety: '1111 2222' });
    await m.settle(5);
    const strip = m.$('.strip-txt')?.textContent ?? '';
    expect(strip).toContain('192.168.15.114:4300');
    expect(strip).toContain('123 456');
  });

  it('says nothing about the network profile unless Windows actually reported one', async () => {
    for (const category of [0, 2]) {
      const m = await mount({ NetworkProfile: async () => ({ category, name: 'HomeWiFi', supported: true }) });
      expect(m.$('#net-public-note'), `category ${category}`).toBeNull();
    }
  });

  it('warns when Windows has the network set to Public', async () => {
    const m = await mount({ NetworkProfile: async () => ({ category: 1, name: 'HomeWiFi', supported: true }) });
    expect(m.$('#net-public-note')).not.toBeNull();
    expect(m.text()).toContain('HomeWiFi');
  });
});

describe('the opening frame', () => {
  it('holds the feed at size until its data lands, then shows it', async () => {
    // Nearby, incoming and recent all arrive asynchronously, so the window used
    // to jump once, moments after opening.
    let release: (v: unknown) => void = () => {};
    const slow = new Promise((r) => (release = r));
    const m = await mount({ ActivityLog: async () => { await slow; return []; } });

    expect(m.$$('.sk').length).toBeGreaterThan(0);
    release(null);
    await m.settle(20);
    expect(m.$$('.sk')).toHaveLength(0);
  });

  it('releases the placeholder even when the data fails to load', async () => {
    const m = await mount({ ActivityLog: async () => { throw new Error('offline'); } });
    await m.settle(20);
    expect(m.$$('.sk')).toHaveLength(0);
  });
});
