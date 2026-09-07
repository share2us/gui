import { defineConfig } from 'vitest/config';

// jsdom, not a browser: these tests are about what the interface SAYS and
// whether the control behind it does that. Layout and computed style are a
// browser's job and are checked by hand, so a headless Chromium here would cost
// a download in CI and prove nothing extra.
export default defineConfig({
  test: {
    environment: 'jsdom',
    include: ['test/**/*.test.ts'],
    restoreMocks: true,
  },
});
