// W-M4: a device's mDNS-advertised fingerprint is attacker-choosable, and the
// sender pins exactly that — so "verified" can mean "verified as the impostor".
// The app asks once per device, then only when the identity changes.
import { describe, it, expect } from 'vitest';
import { mount, click } from './harness';

const peer = {
  name: 'kestrel', addr: '192.168.15.9:4300', dest: 's2u://192.168.15.9:4300?f=AA',
  code: '123 456', mode: 'open', fingerprint: 'AA', isBroadcast: false, fileName: '', fileSize: 0,
};

const withPeer = (check: unknown, extra: Record<string, unknown> = {}) => ({
  PendingPaths: async () => ['/tmp/a.pdf'],
  LanBrowse: async () => [peer],
  PeerCheck: async () => check,
  LanSend: async () => [{ path: '/tmp/a.pdf', ok: true, error: '' }],
  ...extra,
});

/** Stage a file, pick the device, press the primary button. */
async function sendToPeer(m: Awaited<ReturnType<typeof mount>>) {
  await click(m, '[data-send-mode="device"]');
  await click(m, '[data-dest-opt="nearby"]');
  await click(m, '.pick-dev');
  await click(m, '#primary-btn');
}

describe('sending to a nearby device', () => {
  it('goes straight through when the device is the one we sent to before', async () => {
    // The common case must stay one click; asking every time is what teaches
    // people to click through the prompt that matters.
    const m = await mount(withPeer({ status: 'same', code: '123 456', previousCode: '' }));
    await sendToPeer(m);
    expect(m.$('.overlay')).toBeNull();
    expect(m.calls.LanSend?.length).toBe(1);
  });

  it('asks once on first sight, showing the code to compare', async () => {
    const m = await mount(withPeer({ status: 'new', code: '123 456', previousCode: '' }));
    await sendToPeer(m);
    expect(m.calls.LanSend).toBeUndefined();
    expect(m.text()).toContain('First time sending to kestrel');
    expect(m.text()).toContain('123 456');
  });

  it('sends and remembers only after the person confirms', async () => {
    const m = await mount(withPeer({ status: 'new', code: '123 456', previousCode: '' }));
    await sendToPeer(m);
    expect(m.calls.PeerRemember).toBeUndefined(); // not on sight
    await click(m, '#peer-confirm');
    expect(m.calls.PeerRemember?.[0]).toEqual(['kestrel', 'AA']);
    expect(m.calls.LanSend?.length).toBe(1);
  });

  it('sends nothing, and remembers nothing, if the person declines', async () => {
    const m = await mount(withPeer({ status: 'new', code: '123 456', previousCode: '' }));
    await sendToPeer(m);
    await click(m, '#peer-cancel');
    expect(m.calls.LanSend).toBeUndefined();
    expect(m.calls.PeerRemember).toBeUndefined();
  });

  it('says plainly when a known name shows a different identity', async () => {
    const m = await mount(withPeer({ status: 'changed', code: '999 888', previousCode: '123 456' }));
    await sendToPeer(m);
    expect(m.calls.LanSend).toBeUndefined();
    expect(m.text()).toContain('is not the device you sent to before');
    // Both codes, or the change is not legible.
    expect(m.text()).toContain('999 888');
    expect(m.text()).toContain('123 456');
    expect(m.text()).toMatch(/pretending to be it/i);
  });

  it('never blocks a send because the memory itself failed', async () => {
    const m = await mount(withPeer(null, { PeerCheck: async () => { throw new Error('disk gone'); } }));
    await sendToPeer(m);
    expect(m.calls.LanSend?.length).toBe(1);
  });
});

// The pull direction. A broadcast name is claimable by anything on the network,
// and its advertised fingerprint is what gets pinned — so this prompt used to ask
// "download from kestrel?" with no way to know it was kestrel.
describe('downloading from a nearby device', () => {
  const bc = {
    name: 'kestrel', addr: '192.168.15.9:4300', dest: '', code: '123 456', mode: 'open',
    fingerprint: 'BB', isBroadcast: true, fileName: 'holiday.zip', fileSize: 4096,
  };
  const withBroadcast = (check: unknown) => ({
    LanBrowse: async () => [bc],
    PeerCheck: async () => check,
    LanDownload: async () => ({ name: 'holiday.zip', from: 'kestrel', fingerprint: 'BB', trusted: false }),
  });

  it('shows the code to compare on a first download', async () => {
    const m = await mount(withBroadcast({ status: 'new', code: '123 456', previousCode: '' }));
    await click(m, '.dl-btn');
    await m.settle(20);
    expect(m.text()).toContain('First time downloading from this device');
    expect(m.text()).toContain('123 456');
  });

  it('says so, and stops inviting the click, when the identity changed', async () => {
    const m = await mount(withBroadcast({ status: 'changed', code: '999 888', previousCode: '123 456' }));
    await click(m, '.dl-btn');
    await m.settle(20);
    expect(m.text()).toContain('is not the kestrel you downloaded from before');
    expect(m.text()).toContain('999 888');
    expect(m.text()).toContain('123 456');
    // Cancel becomes the emphasised action; Download says "anyway".
    expect(m.$('#dl-cancel')?.className).toContain('btn-accept');
    expect(m.$('#dl-go')?.textContent).toMatch(/anyway/i);
  });

  it('records the device only once the person downloads', async () => {
    const m = await mount(withBroadcast({ status: 'new', code: '123 456', previousCode: '' }));
    await click(m, '.dl-btn');
    await m.settle(20);
    expect(m.calls.PeerRemember).toBeUndefined();
    await click(m, '#dl-go');
    await m.settle(20);
    expect(m.calls.PeerRemember?.[0]).toEqual(['kestrel', 'BB']);
  });

  it('still opens the prompt if the identity lookup fails', async () => {
    const m = await mount({
      LanBrowse: async () => [bc],
      PeerCheck: async () => { throw new Error('nope'); },
    });
    await click(m, '.dl-btn');
    await m.settle(20);
    expect(m.text()).toContain('Download from kestrel');
  });
});
