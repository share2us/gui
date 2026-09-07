// Incoming files: what the list says about a file, and what clicking it does.
import { describe, it, expect } from 'vitest';
import { mount, click } from './harness';

const waiting = { id: 'i1', name: 'report.pdf', size: 1234, from: 'kestrel', at: '', file: '/stage/i1-report.pdf' };
const filed = { id: 'i2', name: 'photo.png', size: 4321, from: 'kestrel', at: '', file: '/home/me/Downloads/photo.png', filed: true, savedTo: '/home/me/Downloads' };

describe('the incoming list', () => {
  it('offers a save control that is always there, not on hover', async () => {
    const m = await mount({ IncomingList: async () => [waiting] });
    expect(m.$$('.save-incoming')).toHaveLength(1);
  });

  it('saves when the row is clicked, not only the small icon', async () => {
    const m = await mount({
      IncomingList: async () => [waiting],
      SaveIncoming: async () => '/home/me/Downloads',
    });
    await click(m, '.inc-row');
    expect(m.calls.SaveIncoming?.[0]).toEqual(['i1']);
  });

  it('still saves from the icon, and only once', async () => {
    const m = await mount({
      IncomingList: async () => [waiting],
      SaveIncoming: async () => '/home/me/Downloads',
    });
    await click(m, '.save-incoming');
    // The row wraps the icon; without stopPropagation this fired twice.
    expect(m.calls.SaveIncoming).toHaveLength(1);
  });

  it('lists a file already saved, and says where it went', async () => {
    // A remembered folder means "stop asking me", not "hide it from me": the
    // file must stay listed so a one-off can go somewhere else.
    const m = await mount({ IncomingList: async () => [filed] });
    expect(m.text()).toContain('photo.png');
    expect(m.text()).toContain('/home/me/Downloads');
  });

  it('does not describe an already-saved file as waiting', async () => {
    const m = await mount({ IncomingList: async () => [filed] });
    expect(m.text()).not.toMatch(/waiting to be saved/i);
  });

  it('says files are waiting when some actually are', async () => {
    const m = await mount({ IncomingList: async () => [waiting, filed] });
    expect(m.text()).toMatch(/waiting to be saved/i);
  });

  it('shows nothing at all when nothing has arrived', async () => {
    const m = await mount({ IncomingList: async () => [] });
    expect(m.$$('.inc-row')).toHaveLength(0);
    expect(m.text()).not.toMatch(/Incoming/);
  });
});
