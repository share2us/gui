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
