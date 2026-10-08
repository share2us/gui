// The Transfers section: live progress for sends, receives, and downloads, shown
// as a bar with a percentage, speed, ETA, and elapsed time. The backend emits the
// bytes; the UI derives the rest. See sectionTransfers()/upsertTransfer in main.ts.
import { describe, it, expect } from 'vitest';
import { mount, click } from './harness';

describe('transfers in progress', () => {
  it('shows a sending transfer with a named bar at the right fill', async () => {
    const m = await mount();
    m.emit('lan-send-progress', { path: '/tmp/movie.iso', name: 'movie.iso', sent: 500, total: 1000 });
    await m.settle(160);
    const text = m.text();
    expect(text).toContain('Transfers');
    expect(text).toContain('movie.iso');
    expect(text).toContain('Sending');
    expect(text).toContain('50%');
    const bar = m.$('.xfer .bar > span') as HTMLElement | null;
    expect(bar?.style.width).toBe('50%');
  });

  it('names a receive from the event and marks the direction', async () => {
    const m = await mount();
    m.emit('lan-recv-progress', { name: 'report.pdf', received: 250, total: 1000 });
    await m.settle(160);
    const text = m.text();
    expect(text).toContain('report.pdf');
    expect(text).toContain('Receiving');
    expect(text).toContain('25%');
  });

  it('shows a download reaching 100% as done', async () => {
    const m = await mount();
    m.emit('lan-dl-progress', { name: 'pkg.zip', received: 1000, total: 1000 });
    await m.settle(160);
    const text = m.text();
    expect(text).toContain('pkg.zip');
    expect(text).toContain('100%');
    expect(text).toMatch(/done/i);
    const bar = m.$('.xfer .bar') as HTMLElement | null;
    expect(bar?.className).toContain('done');
  });

  it('renders nothing when no transfer is in flight', async () => {
    const m = await mount();
    await m.settle(10);
    expect(m.text()).not.toContain('Transfers');
  });
});

describe('transfer controls', () => {
  it('a send can be paused, then resumed', async () => {
    const m = await mount();
    m.emit('lan-send-progress', { id: 'send-1', name: 'movie.iso', sent: 400, total: 1000 });
    await m.settle(160);
    await click(m, '.xfer-act[data-act="pause"]');
    expect(m.calls.PauseSend?.[0]).toEqual(['send-1']);
    await m.settle(5);
    // After pausing, the row shows a Resume control, not a Pause one.
    expect(m.$('.xfer-act[data-act="resume"]')).toBeTruthy();
    expect(m.$('.xfer-act[data-act="pause"]')).toBeNull();
    await click(m, '.xfer-act[data-act="resume"]');
    expect(m.calls.ResumeSend?.[0]).toEqual(['send-1']);
  });

  it('a send ended as paused keeps its row with a resume control', async () => {
    const m = await mount();
    m.emit('lan-send-progress', { id: 'send-2', name: 'big.bin', sent: 300, total: 1000 });
    await m.settle(160);
    m.emit('lan-send-ended', { id: 'send-2', name: 'big.bin', status: 'paused' });
    await m.settle(160);
    expect(m.text()).toContain('paused');
    expect(m.$('.xfer-act[data-act="resume"]')).toBeTruthy();
  });

  it('a cancelled send is removed', async () => {
    const m = await mount();
    m.emit('lan-send-progress', { id: 'send-3', name: 'gone.bin', sent: 300, total: 1000 });
    await m.settle(160);
    m.emit('lan-send-ended', { id: 'send-3', name: 'gone.bin', status: 'cancelled' });
    await m.settle(160);
    expect(m.text()).not.toContain('gone.bin');
  });

  it('an incoming transfer can be paused or cancelled', async () => {
    const m = await mount();
    m.emit('lan-recv-start', { id: 'recv:in.pdf', name: 'in.pdf' });
    m.emit('lan-recv-progress', { id: 'recv:in.pdf', name: 'in.pdf', received: 200, total: 800 });
    await m.settle(160);
    expect(m.$('.xfer-act[data-act="rpause"]')).toBeTruthy();
    await click(m, '.xfer-act[data-act="rcancel"]');
    expect(m.calls.CancelIncoming?.[0]).toEqual(['recv:in.pdf']);
    await m.settle(5);
    expect(m.text()).not.toContain('in.pdf');
  });
});
