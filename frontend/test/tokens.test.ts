// The stylesheet's own rules, checked the only way that lasts.
//
// Sizes were scattered as literals — 84 of them against six token uses — and the
// only reason anyone noticed was an audit. A test notices every time.
import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

// Resolved from the project root: under jsdom, import.meta.url is not a file URL.
const css = readFileSync(resolve(process.cwd(), 'src/style.css'), 'utf8');

/** Strip the :root blocks: that is where a raw value is supposed to live. */
function outsideRoot(text: string): string {
  return text.replace(/:root[^{]*\{[\s\S]*?\n\}/g, '');
}

describe('the stylesheet', () => {
  it('sets no font size as a raw number', () => {
    const stray = outsideRoot(css).match(/font-size:\s*[0-9.]+px/g) ?? [];
    expect(stray, `use a token, or add one to :root for a size the ramp lacks:\n${stray.join('\n')}`).toEqual([]);
  });

  it('names every font stack it uses', () => {
    // Capture the value and test it, rather than a negative lookahead after
    // \s* — that backtracks to zero width and matches every declaration,
    // which is the same shape of regex bug that once mangled the light palette.
    const stray = [...outsideRoot(css).matchAll(/font-family:\s*([^;]+);/g)]
      .map((m) => m[1].trim())
      .filter((v) => !v.startsWith('var('));
    expect(stray, `hand-written stacks:\n${stray.join('\n')}`).toEqual([]);
  });

  it('declares the desktop-only steps it needs, rather than inlining them', () => {
    // These exist because the portal's ramp has no half-steps and nothing below
    // 12px, and this window is 420px wide.
    for (const t of ['--fs-app-sm', '--fs-app-xs', '--font-mono-app']) {
      expect(css, `${t} is used by the app and must stay defined`).toContain(`${t}:`);
    }
  });

  it('defines every custom property it references', () => {
    const defined = new Set([...css.matchAll(/^\s*(--[a-z0-9-]+):/gim)].map((m) => m[1]));
    const used = new Set([...css.matchAll(/var\((--[a-z0-9-]+)/g)].map((m) => m[1]));
    // --wails-drop-target is set by the Wails runtime, not by this file.
    const undef = [...used].filter((u) => !defined.has(u) && !u.startsWith('--wails-'));
    expect(undef, `used but never defined (this is how --muted survived):\n${undef.join('\n')}`).toEqual([]);
  });

  it('gives every colour token a light-mode value', () => {
    // Comments are stripped first: the file's opening line mentions
    // [data-theme="light"] in prose, and matching that swallowed the DARK block
    // and compared it with itself — a test that passed while proving nothing.
    const bare = css.replace(/\/\*[\s\S]*?\*\//g, '');
    const rootBlock = bare.match(/:root\s*\{([\s\S]*?)\n\}/)?.[1] ?? '';
    const lightBlock = bare.match(/:root\[data-theme=["']light["']\]\s*\{([\s\S]*?)\n\}/)?.[1] ?? '';
    expect(lightBlock, 'the light palette must exist').not.toBe('');
    const colourish = /(color|bg|border|shadow|accent|focus|danger|warning|success|surface|overlay)/i;
    const rootTokens = [...rootBlock.matchAll(/^\s*(--[a-z0-9-]+):/gim)].map((m) => m[1]).filter((t) => colourish.test(t));
    const lightTokens = new Set([...lightBlock.matchAll(/^\s*(--[a-z0-9-]+):/gim)].map((m) => m[1]));
    const missing = rootTokens.filter((t) => !lightTokens.has(t));
    expect(missing, `defined once, outside both palettes — these will not flip:\n${missing.join('\n')}`).toEqual([]);
  });
});
