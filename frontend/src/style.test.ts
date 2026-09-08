import { describe, expect, it } from "bun:test";
import { readFileSync, existsSync } from "node:fs";
import { join } from "node:path";

describe("Design System & Theme Tokens", () => {
  const styleCssPath = join(__dirname, "style.css");
  const styleCss = readFileSync(styleCssPath, "utf-8");

  it("defines all required color tokens from Fyne customTheme", () => {
    const lowerCss = styleCss.toLowerCase();
    expect(lowerCss).toContain("--color-truck-500: #6cb944");
    expect(lowerCss).toContain("--color-truck-600: #559b32");
    expect(lowerCss).toContain("--color-truck-100: #e8f7dc");
    expect(lowerCss).toContain("--color-truck-200: #dcf5c3");
    expect(lowerCss).toContain("--color-truck-btn: #dcedd2");
    expect(lowerCss).toContain("--color-truck-input: #f1f8ed");
    expect(lowerCss).toContain("--color-slate-900: #0f172a");
    expect(lowerCss).toContain("--color-slate-800: #1e293b");
    expect(lowerCss).toContain("--color-slate-50: #f8fafc");
    expect(lowerCss).toContain("--color-truck-selection: rgba(108, 185, 68, 0.31)");
  });

  it("defines typography scale tokens matching Fyne header sizes", () => {
    expect(styleCss).toContain("--text-title: 18px");
    expect(styleCss).toContain("--text-subtitle: 13px");
    expect(styleCss).toContain("--text-welcome: 28px");
  });

  it("supports dark mode via prefers-color-scheme and dark: variants", () => {
    expect(styleCss).toContain("prefers-color-scheme: dark");
    expect(styleCss).toContain("@custom-variant dark");
  });

  it("includes tabular-nums utility for CSV table numerals", () => {
    expect(styleCss).toContain("tabular-nums");
    expect(styleCss).toContain("font-variant-numeric: tabular-nums");
  });

  it("honors prefers-reduced-motion for all nonessential motion", () => {
    expect(styleCss).toContain("prefers-reduced-motion: reduce");
  });

  it("has truck.png asset in frontend/src/assets", () => {
    const truckAssetPath = join(__dirname, "assets", "truck.png");
    expect(existsSync(truckAssetPath)).toBe(true);
    const stat = readFileSync(truckAssetPath);
    expect(stat.length).toBeGreaterThan(0);
  });
});
