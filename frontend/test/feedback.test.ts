// Click feedback: a control that starts slow work has to say it started.
//
// The report was that the app looked hung. Nothing was hung: pressing a button
// that opens a native file dialog, probes the subnet or starts a transfer looked
// exactly like pressing one that had not registered the click.
import { describe, it, expect, vi } from 'vitest';
import { mount, click } from './harness';

const named = { name: 'kestrel', addr: '192.168.15.9:4300', dest: 's2u://192.168.15.9:4300?f=AA', code: '123 456', mode: 'open', fingerprint: 'AA', isBroadcast: false, fileName: '', fileSize: 0 };

/** A promise the test decides when to settle, so "still working" is observable. */
function deferred<T>() {
  let resolve!: (v: T) => void;
  const promise = new Promise<T>((r) => { resolve = r; });
  return { promise, resolve };
}

describe('a button running slow work', () => {
  it('says it is working while the work is in flight', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const d = deferred<string[]>();
    const m = await mount({ LanBrowse: async () => [named], PickFiles: () => d.promise });
    const btn = m.$('.send-to')!;
    btn.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    await vi.advanceTimersByTimeAsync(200);
    expect(btn.classList.contains('is-busy')).toBe(true);
    d.resolve([]);
    await vi.advanceTimersByTimeAsync(50);
    expect(btn.classList.contains('is-busy')).toBe(false);
    vi.useRealTimers();
  });

  it('stays quiet for work that finishes before the threshold', async () => {
    // A marker that appears and vanishes inside a tenth of a second reads as a
    // glitch, not as progress; the CSS press state already answers "did that
    // register?". The work here takes 60ms, which is real time but under the
    // threshold, so nothing should ever appear.
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const d = deferred<string[]>();
    const m = await mount({ LanBrowse: async () => [named], PickFiles: () => d.promise });
    const btn = m.$('.send-to')!;
    btn.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    await vi.advanceTimersByTimeAsync(60);
    expect(btn.classList.contains('is-busy')).toBe(false);
    d.resolve([]);
    await vi.advanceTimersByTimeAsync(200);
    expect(btn.classList.contains('is-busy')).toBe(false);
    vi.useRealTimers();
  });

  it('does not leave a control stuck working when the work fails', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const m = await mount({ LanBrowse: async () => [named], PickFiles: async () => { throw new Error('nope'); } });
    const btn = m.$('.send-to')!;
    btn.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    await vi.advanceTimersByTimeAsync(300);
    expect(btn.classList.contains('is-busy')).toBe(false);
    vi.useRealTimers();
  });

  it('marks the rescan button, which repaints the list while it runs', async () => {
    // findNearby renders as it starts, which REPLACES this button. A class put on
    // the node captured at click time would land on something no longer shown, so
    // the button the user is looking at would sit there saying nothing.
    //
    // The first scan is the one at startup; it has to finish, or the click would
    // take the join path below instead and never re-render.
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const d = deferred<unknown[]>();
    let calls = 0;
    const m = await mount({ LanBrowse: () => (++calls === 1 ? Promise.resolve([]) : d.promise) });
    await m.settle(5);
    m.$('#nearby-find')!.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    await vi.advanceTimersByTimeAsync(200);
    expect(m.$('#nearby-find')?.classList.contains('is-busy')).toBe(true);
    d.resolve([]);
    await vi.advanceTimersByTimeAsync(50);
    expect(m.$('#nearby-find')?.classList.contains('is-busy')).toBe(false);
    vi.useRealTimers();
  });

  it('stays marked while it joins a scan that was already running', async () => {
    // Pressing rescan while the automatic check happened to be running used to
    // return immediately and do nothing at all, which looks exactly like a hung
    // app. It now waits on the scan in flight.
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const d = deferred<unknown[]>();
    const m = await mount({ LanBrowse: () => d.promise }); // the startup scan hangs
    m.$('#nearby-find')!.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    await vi.advanceTimersByTimeAsync(200);
    expect(m.$('#nearby-find')?.classList.contains('is-busy')).toBe(true);
    d.resolve([]);
    await vi.advanceTimersByTimeAsync(50);
    expect(m.$('#nearby-find')?.classList.contains('is-busy')).toBe(false);
    vi.useRealTimers();
  });
});

describe('the share button', () => {
  it('keeps its label while working, so the row does not resize', async () => {
    // It used to swap the label to "Sharing…", which changes the button's width
    // and shoves everything beside it (design rule: no layout shift).
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const d = deferred<unknown[]>();
    const m = await mount({
      LanBrowse: async () => [named],
      PickFiles: async () => ['/tmp/a.pdf'],
      Share: () => d.promise,
    });
    await click(m, '.send-to');
    const btn = m.$('#primary-btn') as HTMLButtonElement;
    const before = btn.textContent;
    btn.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    await vi.advanceTimersByTimeAsync(200);
    expect((m.$('#primary-btn') as HTMLButtonElement).textContent).toBe(before);
    d.resolve([]);
    await vi.advanceTimersByTimeAsync(50);
    vi.useRealTimers();
  });
});
