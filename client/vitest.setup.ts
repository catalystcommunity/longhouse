import "@testing-library/jest-dom/vitest";
import { afterEach } from "vitest";
import { cleanup } from "@solidjs/testing-library";

// happy-dom (v20) doesn't expose localStorage, and Node 26's experimental
// global needs a CLI flag we don't want. Provide a small in-memory shim so
// the auth store and its tests behave like a browser. Installed here in
// setupFiles, which run before any test module is imported — so modules
// that read localStorage at import time (auth.ts) see it ready.
if (typeof globalThis.localStorage === "undefined") {
  class MemStorage implements Storage {
    private m = new Map<string, string>();
    get length() {
      return this.m.size;
    }
    clear() {
      this.m.clear();
    }
    getItem(k: string) {
      return this.m.has(k) ? this.m.get(k)! : null;
    }
    setItem(k: string, v: string) {
      this.m.set(k, String(v));
    }
    removeItem(k: string) {
      this.m.delete(k);
    }
    key(i: number) {
      return [...this.m.keys()][i] ?? null;
    }
    [name: string]: unknown;
  }
  Object.defineProperty(globalThis, "localStorage", {
    value: new MemStorage(),
    configurable: true,
  });
}

// Unmount Solid trees and reset storage between tests.
afterEach(() => {
  cleanup();
  localStorage.clear();
});
