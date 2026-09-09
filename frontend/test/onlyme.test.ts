// "Only me": uploading a file to your account without sharing it with anyone.
//
// The link tab offered exactly two things before this — a link anyone holding it
// can open, or a link gated on at least one email address. There was no way to
// simply put a file in Share2Us, which is what people reach for when they want it
// on another of their own machines.
//
// PendingPaths is the entry point these tests use: a non-empty list opens the
// share modal on boot, exactly as the Share verb does.
import { describe, it, expect } from 'vitest';
import { mount, click } from './harness';

const staged = { PendingPaths: async () => ['/tmp/report.pdf'] };

/** Open the share modal on the link tab. */
async function linkTab(overrides: Record<string, unknown> = {}) {
  const m = await mount({ ...staged, ...overrides });
  await click(m, '[data-send-mode="link"]');
  return m;
}

describe('the "Only me" destination', () => {
  it('is offered alongside the two sharing options', async () => {
    const m = await linkTab();
    expect(m.$('[data-dest-opt="public"]')).not.toBeNull();
    expect(m.$('[data-dest-opt="private"]')).not.toBeNull();
    expect(m.$('[data-dest-opt="only-me"]')).not.toBeNull();
  });

  it('says the file is not shared, rather than describing a link', async () => {
    const m = await linkTab();
    await click(m, '[data-dest-opt="only-me"]');
    expect(m.text()).toMatch(/Nobody else can open the link/i);
  });

  // "Create link" would describe the mechanism and hide the meaning: the point of
  // this destination is that the file is uploaded and NOT shared.
  it('names the action on the button', async () => {
    const m = await linkTab();
    await click(m, '[data-dest-opt="only-me"]');
    expect((m.$('#primary-btn') as HTMLButtonElement).textContent).toMatch(/Upload 1 file privately/i);
  });

  // The other private destination refuses to proceed without an email address.
  // This one must not inherit that: nobody is the whole point.
  it('is ready to send with no email addresses entered', async () => {
    const m = await linkTab();
    await click(m, '[data-dest-opt="only-me"]');
    expect((m.$('#primary-btn') as HTMLButtonElement).disabled).toBe(false);
  });
});

describe('what it sends to the backend', () => {
  it('asks for the only-me target and no recipients', async () => {
    const m = await linkTab({
      Share: async () => [{ path: '/tmp/report.pdf', ok: true, link: 'https://s.test/pub-1', publicId: 'pub-1' }],
    });
    await click(m, '[data-dest-opt="only-me"]');
    await click(m, '#primary-btn');

    const sent = (m.calls.Share ?? [])[0]?.[0] as { target: string; recipients?: string[] } | undefined;
    // Assert the call happened before asserting its shape: `sent?.x` on an
    // undefined `sent` would let a never-sent request pass silently.
    expect(sent).toBeDefined();
    expect(sent?.target).toBe('only-me');
    // A recipient list left over from switching destinations must never ride
    // along: "only me" means nobody.
    expect(sent?.recipients ?? []).toEqual([]);
  });

  it('reports it as uploaded rather than shared', async () => {
    const m = await linkTab({
      Share: async () => [{ path: '/tmp/report.pdf', ok: true, link: 'https://s.test/pub-1', publicId: 'pub-1' }],
    });
    await click(m, '[data-dest-opt="only-me"]');
    await click(m, '#primary-btn');
    await m.settle(10);

    expect(m.text()).toMatch(/Uploaded privately/i);
    expect(m.text()).not.toMatch(/is shared\./i);
  });
});

describe('a login is still required', () => {
  it('blocks the send when signed out, like the other cloud destinations', async () => {
    const m = await linkTab({
      Status: async () => ({
        loggedIn: false, email: '', isApiToken: false, canReceive: false,
        shellInstalled: false, autostartEnabled: false, discoverable: true,
      }),
    });
    await click(m, '[data-dest-opt="only-me"]');
    expect((m.$('#primary-btn') as HTMLButtonElement).disabled).toBe(true);
    expect(m.text()).toMatch(/Login to share to the cloud/i);
  });
});
