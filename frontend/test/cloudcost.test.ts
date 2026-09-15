// The cloud-cost prompt (ADR-040 / P1-5).
//
// A device send lands here when the target machines are NOT on this network, so
// the file has to be uploaded. The owner's rule is that a user must know the
// cloud is the paid part BEFORE it happens: "add option (keep in cloud [storage
// counts towards your quota]) to user so user will know". A silent upload is the
// thing this exists to prevent, which is why it asks rather than announces.
import { describe, it, expect } from 'vitest';
import { mount, click } from './harness';

const staged = { PendingPaths: async () => ['/tmp/report.pdf'] };

async function asked(data: Record<string, unknown>) {
  const m = await mount({ ...staged });
  m.emit('cloud-fallback', { id: 'cloud1', ...data });
  await m.settle(5);
  return m;
}

describe('the cloud-cost prompt', () => {
  it('names the devices and the size, and says it counts against the quota', async () => {
    const m = await asked({ deviceNames: ['laptop'], sizeBytes: 5_000_000, directWasPossible: true });
    expect(m.text()).toMatch(/upload/i);
    expect(m.text()).toContain('laptop');
    expect(m.text()).toMatch(/quota/i);
  });

  // ONE upload covers every device — the content key is sealed per device — so
  // the size must never be multiplied by the number of machines. Overstating it
  // would push people off something cheaper than they think.
  it('reports one file size however many devices are selected', async () => {
    const m = await asked({ deviceNames: ['laptop', 'desktop', 'phone'], sizeBytes: 1_000_000, directWasPossible: false });
    const text = m.text();
    expect(text).toContain('laptop');
    expect(text).toContain('desktop');
    // 1 MB once, not 3 MB: no rendering of the tripled figure anywhere.
    expect(text).not.toMatch(/3(\.0)?\s*MB/i);
  });

  // Advice you cannot act on is the same fault as telling someone to install the
  // app on their browser, so the hint is shown only when one of THESE devices
  // could actually have taken the file directly.
  //
  // Scoped to the dialog on purpose: the share view's own settings copy contains
  // "Discoverable on local network", so a whole-page assertion passes whatever
  // the dialog says. It did, until this was narrowed.
  it('offers the discoverable hint only when a direct send was possible', async () => {
    const yes = await asked({ deviceNames: ['laptop'], sizeBytes: 10, directWasPossible: true });
    expect(yes.$('.overlay-card')?.textContent ?? '').toMatch(/discoverable/i);

    const no = await asked({ deviceNames: ['old-pc'], sizeBytes: 10, directWasPossible: false });
    expect(no.$('.overlay-card')?.textContent ?? '').not.toMatch(/discoverable/i);
  });

  it('uploads when accepted', async () => {
    const m = await asked({ deviceNames: ['laptop'], sizeBytes: 10, directWasPossible: false });
    await click(m, '#cloud-go');
    expect(m.calls.RespondCloudFallback?.[0]).toEqual(['cloud1', true]);
  });

  it('does not upload when cancelled', async () => {
    const m = await asked({ deviceNames: ['laptop'], sizeBytes: 10, directWasPossible: false });
    await click(m, '#cloud-cancel');
    expect(m.calls.RespondCloudFallback?.[0]).toEqual(['cloud1', false]);
  });

  // Dismissing a dialog must never be the gesture that spends money.
  it('treats Escape as a decision NOT to upload', async () => {
    const m = await asked({ deviceNames: ['laptop'], sizeBytes: 10, directWasPossible: false });
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    await new Promise((r) => setTimeout(r, 0));
    expect(m.calls.RespondCloudFallback?.[0]).toEqual(['cloud1', false]);
  });

  // The send is blocked in Go until this is answered, so the dialog must go away
  // on the first answer — a second click cannot reach a request already resolved.
  it('closes once answered', async () => {
    const m = await asked({ deviceNames: ['laptop'], sizeBytes: 10, directWasPossible: false });
    await click(m, '#cloud-go');
    expect(m.$('#cloud-go')).toBeNull();
  });
});
