// A stub backend and a fresh copy of the app, per test.
//
// This file exists because the same throwaway harness was rewritten four times
// in one day while chasing field reports. main.ts boots on import and talks to
// window.go.main.App, so mounting it is: put #app in the document, install the
// stubs, then import it fresh.
import { vi } from 'vitest';

/** Calls the stub recorded, so a test can assert what the UI asked the backend to do. */
export type Calls = Record<string, unknown[][]>;

export type Mounted = {
  /** Backend calls, by method name, in order. */
  calls: Calls;
  /** URLs the app asked the system browser to open. */
  opened: string[];
  /** Wails events the app emitted a listener for, so a test can fire one. */
  emit: (name: string, data?: unknown) => void;
  /** Re-render has no hook, so tests wait for the app to settle instead. */
  settle: (ms?: number) => Promise<void>;
  $: (sel: string) => HTMLElement | null;
  $$: (sel: string) => HTMLElement[];
  text: () => string;
};

/** Everything the app calls, with defaults that make a logged-in, idle window. */
function defaults() {
  return {
    Status: async () => ({
      loggedIn: true, email: 'someone@example.com', isApiToken: false, canReceive: true,
      shellInstalled: false, autostartEnabled: false, discoverable: true,
    }),
    PendingPaths: async () => [] as string[],
    LanBrowse: async () => [] as unknown[],
    LocalAddresses: async () => ['192.168.15.114'],
    NetworkProfile: async () => ({ category: 0, name: '', supported: true }),
    Activity: async () => [] as unknown[],
    ActivityLog: async () => [] as unknown[],
    IncomingList: async () => [] as unknown[],
    IncomingFolder: async () => '',
    GetScanInterval: async () => 60,
    TrustedDevices: async () => [] as unknown[],
    ListTrusted: async () => [] as unknown[],
    CheckUpdate: async () => null,
    UpdateChannel: async () => 'stable',
    IsStoreManaged: async () => false,
    BuildVersion: async () => '20260908010101',
    ClipboardSuggestion: async () => null,
  };
}

export async function mount(overrides: Record<string, unknown> = {}): Promise<Mounted> {
  document.body.innerHTML = '<div id="app"></div>';
  // jsdom has no layout, so it has no scrollIntoView. The app calls it when it
  // opens Settings; without this the click throws and the failure looks like a
  // bug in the code under test rather than a gap in the environment.
  if (!(Element.prototype as any).scrollIntoView) {
    (Element.prototype as any).scrollIntoView = () => {};
  }

  const calls: Calls = {};
  const opened: string[] = [];
  const listeners: Record<string, (d: unknown) => void> = {};
  const impl: Record<string, unknown> = { ...defaults(), ...overrides };

  const App = new Proxy({}, {
    get: (_t, name: string) => {
      const fn = impl[name];
      return (...args: unknown[]) => {
        (calls[name] ||= []).push(args);
        return typeof fn === 'function' ? (fn as (...a: unknown[]) => unknown)(...args) : Promise.resolve(null);
      };
    },
  });
  (globalThis as any).window.go = { main: { App } };
  (globalThis as any).window.runtime = new Proxy({}, {
    get: (_t, name: string) => {
      if (name === 'EventsOn') return (n: string, f: (d: unknown) => void) => { listeners[n] = f; };
      if (name === 'BrowserOpenURL') return (u: string) => opened.push(u);
      if (name === 'ClipboardSetText') return () => {};
      return () => {};
    },
  });

  // A fresh module each time: main.ts keeps its state in a module-level object.
  vi.resetModules();
  await import('../src/main.ts');

  const settle = async (ms = 0) => {
    await new Promise((r) => setTimeout(r, ms));
    await Promise.resolve();
  };
  await settle(5);

  return {
    calls, opened,
    emit: (name, data) => listeners[name]?.(data),
    settle,
    $: (sel) => document.querySelector(sel),
    $$: (sel) => Array.from(document.querySelectorAll(sel)),
    text: () => (document.querySelector('#app') as HTMLElement)?.innerText ?? document.body.textContent ?? '',
  };
}

/** Click something, then let the app re-render. */
export async function click(m: Mounted, sel: string) {
  const el = m.$(sel);
  if (!el) throw new Error(`nothing matches ${sel}`);
  el.dispatchEvent(new MouseEvent('click', { bubbles: true }));
  await m.settle(5);
}

/** Type into an input the way a person would, firing the input event. */
export async function type(m: Mounted, sel: string, value: string) {
  const el = m.$(sel) as HTMLInputElement | null;
  if (!el) throw new Error(`nothing matches ${sel}`);
  el.value = value;
  el.dispatchEvent(new Event('input', { bubbles: true }));
  await m.settle(5);
}
