// Reaching the app without a mouse.
//
// Choosing where a file goes was drawn as plain <div>s with click handlers: no
// role, no tabindex, nothing for the keyboard to land on. The feature was not
// merely awkward from the keyboard, it was unreachable.
import { describe, it, expect } from 'vitest';
import { mount, click } from './harness';

const oneFile = { PendingPaths: async () => ['/tmp/a.pdf'] };

async function shareView(over = {}) {
  const m = await mount({ ...oneFile, ...over });
  await click(m, '[data-send-mode="device"]');
  return m;
}

/** Press a key on an element the way a keyboard user would. */
async function press(m: Awaited<ReturnType<typeof mount>>, sel: string, key: string) {
  const el = m.$(sel)!;
  el.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true }));
  await m.settle(5);
}

describe('the destination picker', () => {
  it('can be focused', async () => {
    const m = await shareView();
    for (const el of m.$$('.dest-opt')) {
      expect(el.getAttribute('tabindex'), el.dataset.destOpt).toBe('0');
    }
  });

  it('says what it is, and which one is chosen', async () => {
    const m = await shareView();
    const opts = m.$$('.dest-opt');
    expect(opts.every((e) => e.getAttribute('role') === 'radio')).toBe(true);
    expect(opts.filter((e) => e.getAttribute('aria-checked') === 'true')).toHaveLength(1);
  });

  it('is chosen with Enter', async () => {
    const m = await shareView();
    await press(m, '[data-dest-opt="broadcast"]', 'Enter');
    expect(m.$('[data-dest-opt="broadcast"]')?.getAttribute('aria-checked')).toBe('true');
  });

  it('is chosen with Space', async () => {
    const m = await shareView();
    await press(m, '[data-dest-opt="broadcast"]', ' ');
    expect(m.$('[data-dest-opt="broadcast"]')?.getAttribute('aria-checked')).toBe('true');
  });

  it('ignores keys that are not an activation', async () => {
    const m = await shareView();
    const before = m.$('[data-dest-opt="nearby"]')?.getAttribute('aria-checked');
    await press(m, '[data-dest-opt="broadcast"]', 'a');
    expect(m.$('[data-dest-opt="nearby"]')?.getAttribute('aria-checked')).toBe(before);
  });
});

describe('broadcast access modes', () => {
  it('are reachable and chosen from the keyboard', async () => {
    const m = await shareView();
    await click(m, '[data-dest-opt="broadcast"]');
    const modes = m.$$('.mode');
    expect(modes.length).toBeGreaterThan(0);
    expect(modes.every((e) => e.getAttribute('role') === 'radio' && e.getAttribute('tabindex') === '0')).toBe(true);

    await press(m, '[data-bc-mode="trusted"]', 'Enter');
    expect(m.$('[data-bc-mode="trusted"]')?.getAttribute('aria-checked')).toBe('true');
  });
});

describe('icon-only buttons', () => {
  it('all say what they do', async () => {
    // A button whose whole label is a glyph is silent to a screen reader.
    const m = await shareView();
    for (const b of m.$$('button')) {
      const label = (b.textContent ?? '').trim();
      const named = b.getAttribute('aria-label') || b.getAttribute('title');
      const isGlyphOnly = label.length > 0 && label.length <= 2 && !/[a-z0-9]/i.test(label);
      if (isGlyphOnly) {
        expect(named, `button "${label}" (#${b.id || b.className}) has no accessible name`).toBeTruthy();
      }
    }
  });
});
