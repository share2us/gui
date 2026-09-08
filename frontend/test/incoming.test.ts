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

  // "add a option to cancel incoming request" — Discard was bound but unreachable,
  // so a file you did not want could only be waited out for a week.
  it('offers a way to get rid of a file you do not want', async () => {
    const m = await mount({ IncomingList: async () => [waiting] });
    expect(m.$$('.drop-incoming')).toHaveLength(1);
  });

  it('asks before deleting a file that has not been saved anywhere', async () => {
    const m = await mount({ IncomingList: async () => [waiting], DiscardIncoming: async () => null });
    await click(m, '.drop-incoming');
    // Nothing is destroyed on the first click: the ✕ sits beside the button that
    // saves, and deleting the only copy has no undo.
    expect(m.calls.DiscardIncoming).toBeUndefined();
    expect(m.text()).toMatch(/Delete without saving/i);
    await click(m, '#drop-confirm');
    expect(m.calls.DiscardIncoming?.[0]).toEqual(['i1']);
  });

  it('keeps the file when the confirmation is declined', async () => {
    const m = await mount({ IncomingList: async () => [waiting], DiscardIncoming: async () => null });
    await click(m, '.drop-incoming');
    await click(m, '#drop-cancel');
    expect(m.calls.DiscardIncoming).toBeUndefined();
    expect(m.text()).not.toMatch(/Delete without saving/i);
  });

  it('does not ask twice to remove an already-saved file from the list', async () => {
    // Nothing is destroyed here — the file stays in the folder the user chose —
    // so a confirmation would be nagging about a reversible action.
    const m = await mount({ IncomingList: async () => [filed], DiscardIncoming: async () => null });
    await click(m, '.drop-incoming');
    expect(m.text()).not.toMatch(/Delete without saving/i);
    expect(m.calls.DiscardIncoming?.[0]).toEqual(['i2']);
  });

  it('does not save the file when the cancel control is clicked', async () => {
    const m = await mount({
      IncomingList: async () => [waiting],
      SaveIncoming: async () => '/home/me/Downloads',
      DiscardIncoming: async () => null,
    });
    // The row saves on click and wraps the ✕; without stopPropagation, asking to
    // delete a file would first open a dialog offering to save it.
    await click(m, '.drop-incoming');
    expect(m.calls.SaveIncoming).toBeUndefined();
  });

  it('shows nothing at all when nothing has arrived', async () => {
    const m = await mount({ IncomingList: async () => [] });
    expect(m.$$('.inc-row')).toHaveLength(0);
    expect(m.text()).not.toMatch(/Incoming/);
  });
});

// "After accepting the file it should open download prompt ... instead of user
// accepting then file and then clicking on download button to save it this
// causes confusion as to where file went."
describe('a file that has just arrived', () => {
  it('asks where to put it straight away, without waiting to be clicked', async () => {
    const m = await mount({
      IncomingList: async () => [waiting],
      SaveIncoming: async () => '/home/me/Downloads',
    });
    expect(m.calls.SaveIncoming).toBeUndefined(); // nothing has arrived yet
    m.emit('incoming-arrived', { id: 'i1', name: 'report.pdf', from: 'kestrel' });
    await m.settle();
    expect(m.calls.SaveIncoming?.[0]).toEqual(['i1']);
  });

  it('asks once per file when several land together, not all at once', async () => {
    // Two native dialogs racing would mean answering the second for a file the
    // user was never shown.
    let resolveFirst: (v: string) => void = () => {};
    const first = new Promise<string>((r) => { resolveFirst = r; });
    let n = 0;
    const m = await mount({
      IncomingList: async () => [waiting],
      SaveIncoming: async () => (++n === 1 ? first : '/home/me/Downloads'),
    });
    m.emit('incoming-arrived', { id: 'i1' });
    m.emit('incoming-arrived', { id: 'i2' });
    await m.settle();
    expect(m.calls.SaveIncoming).toHaveLength(1); // second is queued behind the first
    resolveFirst('/home/me/Downloads');
    await m.settle();
    expect(m.calls.SaveIncoming?.map((c) => c[0])).toEqual(['i1', 'i2']);
  });
});
