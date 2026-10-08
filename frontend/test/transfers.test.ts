// The Transfers section: live progress for sends, receives, and downloads, shown
// as a bar with a percentage, speed, ETA, and elapsed time. The backend emits the
// bytes; the UI derives the rest. See sectionTransfers()/upsertTransfer in main.ts.
import { describe, it, expect } from 'vitest';
import { mount } from './harness';

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
