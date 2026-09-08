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
  it('shows its own address and its verify code', async () => {
    // So the person reading someone else's device list can tell which entry is
    // theirs, without going to look it up in the operating system.
    const m = await mount();
    m.emit('lan-discoverable', { address: '192.168.15.114:4300', code: '123 456', safety: '1111 2222' });
    await m.settle(5);
    expect(m.$('.strip-self')?.textContent ?? '').toContain('192.168.15.114:4300');
    expect(m.$('.strip-txt')?.textContent ?? '').toContain('123 456');
  });

  it('shows its address even when it is not discoverable', async () => {
    // The moment you are most likely to be reading this is while working out why
    // the other laptop cannot see you — which is exactly when discoverable is
    // off. Keeping the address inside the "Discoverable" sentence took it away
    // at that moment.
    const m = await mount({
      Status: async () => ({
        loggedIn: true, email: 'someone@example.com', isApiToken: false, canReceive: true,
        shellInstalled: false, autostartEnabled: false, discoverable: false,
      }),
    });
    await m.settle(5);
    expect(m.$('.strip-txt')?.textContent ?? '').toMatch(/not discoverable/i);
    expect(m.$('.strip-self')?.textContent ?? '').toContain('192.168.15.114');
  });

  it('keeps the address slot present when there is no address, so the strip does not reflow', async () => {
    const m = await mount({ LocalAddresses: async () => [] });
    await m.settle(5);
    expect(m.$('.strip-self')).not.toBeNull();
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

// A scan-discovered device used to be a bare address, because a TLS probe
// carried no name and mDNS — the only source of one — is exactly what fails on a
// Public firewall profile, across subnets, and for every tailnet peer. The name
// now rides in the device's signed card.
const carded = {
  ...named,
  name: 'kestrel',
  address: '192.168.15.9',
  identity: 'ID-KESTREL',
  viaScan: true,
};

describe('a device that published a card', () => {
  it('is shown by name even though it was found by probing', async () => {
    const m = await mount({ LanBrowse: async () => [carded] });
    expect(m.text()).toContain('kestrel');
    expect(m.text()).not.toMatch(/name not announced/i);
  });

  it('still shows the address, so the machine in front of you is identifiable', async () => {
    const m = await mount({ LanBrowse: async () => [carded] });
    expect(m.text()).toContain('192.168.15.9:4300');
  });

  it('can be given a local name', async () => {
    const m = await mount({ LanBrowse: async () => [carded], PeerAlias: async () => null });
    await click(m, '.rename-peer');
    const input = m.$('#rename-input') as HTMLInputElement;
    input.value = 'Study laptop';
    await click(m, '#rename-save');
    expect(m.calls.PeerAlias?.[0]).toEqual(['ID-KESTREL', 'Study laptop']);
  });

  it('names the device by its identity, never by its address or session key', async () => {
    // The address moves with DHCP and the certificate is regenerated every time
    // the device restarts. An alias attached to either would follow the wrong
    // machine, or be lost on a reboot.
    const m = await mount({ LanBrowse: async () => [carded], PeerAlias: async () => null });
    await click(m, '.rename-peer');
    await click(m, '#rename-save');
    const key = m.calls.PeerAlias?.[0]?.[0];
    expect(key).toBe('ID-KESTREL');
    expect(key).not.toBe('AA');
    expect(key).not.toContain('192.168');
  });

  it('clears the local name when the field is emptied', async () => {
    const m = await mount({
      LanBrowse: async () => [{ ...carded, name: 'Study laptop', aliased: true }],
      PeerAlias: async () => null,
    });
    await click(m, '.rename-peer');
    (m.$('#rename-input') as HTMLInputElement).value = '   ';
    await click(m, '#rename-save');
    expect(m.calls.PeerAlias?.[0]).toEqual(['ID-KESTREL', '']);
  });

  it("says a name is yours rather than the device's own", async () => {
    const m = await mount({ LanBrowse: async () => [{ ...carded, name: 'Study laptop', aliased: true }] });
    expect(m.text()).toMatch(/the name you gave it/i);
  });
});

describe('a device with no proven identity', () => {
  it('cannot be named, and says why instead of silently doing nothing', async () => {
    // There would be nothing stable to attach the name to: keyed on an address,
    // it would move to whatever holds that address next.
    const m = await mount({ LanBrowse: async () => [scanned] });
    const btn = m.$('.rename-peer') as HTMLButtonElement;
    expect(btn.disabled).toBe(true);
    expect(btn.title).toMatch(/stable identity/i);
  });

  it('keeps the control present so the row does not reflow', async () => {
    // Design rule: no layout shift. A button that disappears for some devices
    // moves every row beside it between refreshes.
    const m = await mount({ LanBrowse: async () => [scanned, carded] });
    expect(m.$$('.rename-peer')).toHaveLength(2);
  });
});

// The bug this fixes, and the reason the identity had to reach discovery at all.
// PeerCheck was fed the per-session certificate, which is regenerated every time
// a device restarts — so an honest reboot looked like "this is not the device you
// sent to last time", firing the one prompt W-M4 built to be meaningful on the
// one case it must never fire on. Enough of those and people click through it.
describe('remembering a device', () => {
  it('remembers it by its stable identity, not its session certificate', async () => {
    const m = await mount({
      LanBrowse: async () => [carded],
      PickFiles: async () => ['/tmp/a.pdf'],
      PeerCheck: async () => ({ status: 'same', code: '123 456', previousCode: '' }),
      LanSend: async () => [{ path: '/tmp/a.pdf', ok: true, error: '' }],
    });
    await click(m, '.send-to');
    await click(m, '#primary-btn');
    await m.settle(20);
    expect(m.calls.PeerCheck?.[0]).toEqual(['kestrel', 'ID-KESTREL']);
    expect(m.calls.PeerCheck?.[0]?.[1]).not.toBe('AA'); // 'AA' is the session cert
  });

  it('falls back to the certificate for a device with no card', async () => {
    // An older peer publishes no identity. It is still checked — just with the
    // only value it offers, exactly as before.
    const m = await mount({
      LanBrowse: async () => [named],
      PickFiles: async () => ['/tmp/a.pdf'],
      PeerCheck: async () => ({ status: 'same', code: '123 456', previousCode: '' }),
      LanSend: async () => [{ path: '/tmp/a.pdf', ok: true, error: '' }],
    });
    await click(m, '.send-to');
    await click(m, '#primary-btn');
    await m.settle(20);
    expect(m.calls.PeerCheck?.[0]).toEqual(['kestrel', 'AA']);
  });
});
