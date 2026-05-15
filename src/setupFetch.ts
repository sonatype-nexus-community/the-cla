// Early fetch polyfill — runs before modules load (setupFiles, not setupFilesAfterEnv)
// This must run before any module-scope code that calls fetch()

// Node 18+ has fetch globally; ensure it's available in older envs / jsdom
if (globalThis.fetch === undefined) {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  (globalThis as any).fetch = () =>
    Promise.reject(new Error('fetch: no server in test environment'));
}
