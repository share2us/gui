// Every test here is a bug that reached a user.
//
// They share one shape: the interface asserted something the code did not do.
// A compiler cannot check a promise and neither can a render pass, so each test
// reads the label and then checks the behaviour behind it.
import { describe, it, expect } from 'vitest';
import { mount, click, type } from './harness';

const twoFiles = { PendingPaths: async () => ['/tmp/a.pdf', '/tmp/b.pdf'] };
const oneFile = { PendingPaths: async () => ['/tmp/a.pdf'] };

/** Move the send flow to a tab and a destination. */
async function choose(m: Awaited<ReturnType<typeof mount>>, mode: string, dest: string) {
  await click(m, `[data-send-mode="${mode}"]`);
  await click(m, `[data-dest-opt="${dest}"]`);
}

const primary = (m: Awaited<ReturnType<typeof mount>>) =>
  m.$('#primary-btn') as HTMLButtonElement;

describe('"Only these people"', () => {
  it('asks who, instead of silently making an unrestricted link', async () => {
    const m = await mount(oneFile);
    await choose(m, 'link', 'private');
    expect(m.$('#recipients')).not.toBeNull();
  });

  it('does not offer the box on a public link, which has no recipients', async () => {
    const m = await mount(oneFile);
    await choose(m, 'link', 'public');
    expect(m.$('#recipients')).toBeNull();
  });

  it('will not create the link until at least one address is given', async () => {
    const m = await mount(oneFile);
    await choose(m, 'link', 'private');
    expect(primary(m).disabled).toBe(true);
    expect(m.$('.foot-reason')?.textContent).toMatch(/email address/i);

    await type(m, '#recipients', 'someone@example.com');
    await click(m, '[data-dest-opt="private"]'); // force a repaint
    expect(primary(m).disabled).toBe(false);
  });

  it('keeps what was typed across a repaint', async () => {
    const m = await mount(oneFile);
    await choose(m, 'link', 'private');
    await type(m, '#recipients', 'a@b.c');
    await click(m, '[data-dest-opt="private"]');
    expect((m.$('#recipients') as HTMLInputElement).value).toBe('a@b.c');
  });
});

describe('broadcast', () => {
  it('never names more files than it can send', async () => {
    const m = await mount(twoFiles);
    await choose(m, 'device', 'broadcast');
    // It used to read "Broadcast 2 files to everyone nearby" and send the first.
    expect(primary(m).textContent).not.toMatch(/2 files to everyone/i);
    expect(primary(m).disabled).toBe(true);
    expect(m.$('.foot-reason')?.textContent).toMatch(/one file at a time/i);
  });

  it('is available for a single file', async () => {
    const m = await mount(oneFile);
    await choose(m, 'device', 'broadcast');
    expect(primary(m).disabled).toBe(false);
    expect(primary(m).textContent).toMatch(/1 file/);
  });

  it('never leaves the button enabled while the reason says it is blocked', async () => {
    // canPrimary and footerReason each kept their own copy of the rules and had
    // already drifted. They are one source now; this holds them together.
    for (const [mode, dest, files] of [
      ['device', 'broadcast', twoFiles],
      ['link', 'private', oneFile],
      ['device', 'nearby', oneFile],
    ] as const) {
      const m = await mount(files);
      await choose(m, mode, dest);
      const reason = m.$('.foot-reason')?.textContent?.trim() ?? '';
      expect(primary(m).disabled, `${dest}: reason=${JSON.stringify(reason)}`).toBe(reason !== '');
    }
  });
});

describe('the address field under a device', () => {
  it('asks for an address, not the verify code it rejects', async () => {
    const m = await mount(oneFile);
    await choose(m, 'device', 'nearby');
    const input = m.$('#net-dest') as HTMLInputElement;
    expect(input).not.toBeNull();
    expect(input.placeholder).not.toMatch(/code/i);
    expect(m.text()).not.toMatch(/or a code/i);
  });

  it('has a visible way to submit, disabled until something is typed', async () => {
    const m = await mount(oneFile);
    await choose(m, 'device', 'nearby');
    const use = m.$('#net-dest-use') as HTMLButtonElement;
    expect(use).not.toBeNull();
    expect(use.disabled).toBe(true);

    await type(m, '#net-dest', '192.168.15.9');
    // Enabled in place: a re-render here would take the caret out of the field.
    expect((m.$('#net-dest-use') as HTMLButtonElement).disabled).toBe(false);
  });
});

describe('Settings', () => {
  it('closes again, having been opened', async () => {
    const m = await mount();
    const details = () => m.$('details.settings') as HTMLDetailsElement;
    await click(m, '#open-settings');
    expect(details().open).toBe(true);
    await click(m, '#open-settings');
    expect(details().open).toBe(false);
  });
});

describe('project links', () => {
  it('offers the home page and the source, and opens them outside the app', async () => {
    const m = await mount();
    const hrefs = m.$$('.ext-link').map((e) => e.dataset.href);
    expect(hrefs).toContain('https://share2.us');
    expect(hrefs).toContain('https://github.com/share2us');

    await click(m, '.ext-link[data-href="https://github.com/share2us"]');
    expect(m.opened).toContain('https://github.com/share2us');
  });
});

describe('software update', () => {
  const ready = { available: true, current: '20260908', latest: '20260909', prerelease: false, assetUrl: '', assetName: '', page: '', channel: 'stable' };

  it('marks the icon rather than inserting a bar into the layout', async () => {
    // The dot was added and the banner was never removed, so the app announced an
    // update twice and pushed the whole body down when the check came back.
    const m = await mount({ CheckUpdate: async () => ready });
    await m.settle(30);
    expect(m.$('#check-update')?.classList.contains('has-dot')).toBe(true);
    expect(m.$('.update-bar')).toBeNull();
  });

  it('offers Install in a panel, opened from the icon', async () => {
    const m = await mount({ CheckUpdate: async () => ready });
    await m.settle(30);
    expect(m.$('#upd-panel')).toBeNull(); // nothing until asked for
    await click(m, '#check-update');
    expect(m.$('#upd-panel')).not.toBeNull();
    expect(m.$('#apply-update')).not.toBeNull();
    expect(m.text()).toContain('20260909');
  });

  it('closes again', async () => {
    const m = await mount({ CheckUpdate: async () => ready });
    await m.settle(30);
    await click(m, '#check-update');
    await click(m, '#upd-close');
    expect(m.$('#upd-panel')).toBeNull();
  });

  it('says so when there is nothing to install', async () => {
    const m = await mount({ CheckUpdate: async () => null });
    await click(m, '#check-update');
    await m.settle(30);
    expect(m.text()).toMatch(/up to date/i);
    expect(m.$('#apply-update')).toBeNull();
  });
});

describe('the reason under the primary button', () => {
  it('keeps its line even when there is nothing to say', async () => {
    // It used to appear and vanish as files were added and a device chosen,
    // resizing the sticky footer and the list above it.
    const m = await mount({ PendingPaths: async () => ['/tmp/a.pdf'] });
    await click(m, '[data-send-mode="link"]');
    await click(m, '[data-dest-opt="public"]');
    expect(m.$('.foot-reason')).not.toBeNull();
    expect(m.$('.foot-reason')?.textContent?.trim()).toBe('');
  });
});
