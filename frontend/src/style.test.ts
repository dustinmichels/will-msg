import { describe, expect, it } from "bun:test";
import { readFileSync, existsSync } from "node:fs";
import { join } from "node:path";

describe("Design System & Theme Tokens", () => {
  const styleCssPath = join(__dirname, "style.css");
  const styleCss = readFileSync(styleCssPath, "utf-8");

  it("defines all required color tokens from Fyne customTheme", () => {
    expect(styleCss).toContain("--color-truck-500: #6CB944");
    expect(styleCss).toContain("--color-truck-600: #559B32");
    expect(styleCss).toContain("--color-truck-100: #E8F7DC");
    expect(styleCss).toContain("--color-truck-200: #DCF5C3");
    expect(styleCss).toContain("--color-truck-btn: #DCEDD2");
    expect(styleCss).toContain("--color-truck-input: #F1F8ED");
    expect(styleCss).toContain("--color-slate-900: #0F172A");
    expect(styleCss).toContain("--color-slate-800: #1E293B");
    expect(styleCss).toContain("--color-slate-50: #F8FAFC");
    expect(styleCss).toContain("--color-truck-selection: rgba(108, 185, 68, 0.31)");
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
