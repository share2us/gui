import { describe, it, expect } from 'vitest';
import { mount } from './harness';

describe('the window comes up', () => {
  it('renders and asks the backend who is signed in', async () => {
    const m = await mount();
    expect(m.calls.Status?.length).toBeGreaterThan(0);
    expect(m.text()).toContain('someone@example.com');
  });
});
