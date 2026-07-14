import assert from "node:assert/strict";
import test from "node:test";

import {
  SingleserveError,
  TAB_HEADER,
  TOKEN_HEADER,
  TOKEN_STORAGE_KEY,
  connect,
} from "./singleserve.js";

class MemoryStorage {
  values = new Map();

  getItem(key) {
    return this.values.get(key) ?? null;
  }

  setItem(key, value) {
    this.values.set(key, String(value));
  }
}

class FakeWindow {
  constructor(href) {
    this.location = { href };
    this.sessionStorage = new MemoryStorage();
    this.history = {
      state: { preserved: true },
      replacements: [],
      replaceState: (state, unused, url) => this.history.replacements.push({ state, unused, url }),
    };
    this.listeners = new Map();
    this.setTimeout = globalThis.setTimeout.bind(globalThis);
    this.clearTimeout = globalThis.clearTimeout.bind(globalThis);
  }

  addEventListener(name, callback) {
    const callbacks = this.listeners.get(name) ?? [];
    callbacks.push(callback);
    this.listeners.set(name, callbacks);
  }

  removeEventListener(name, callback) {
    this.listeners.set(name, (this.listeners.get(name) ?? []).filter((item) => item !== callback));
  }

  dispatch(name) {
    for (const callback of this.listeners.get(name) ?? []) callback();
  }
}

function ok(payload = { ok: true }, status = 200) {
  return new Response(JSON.stringify(payload), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

async function withWindow(href, run) {
  const previous = globalThis.window;
  const browser = new FakeWindow(href);
  globalThis.window = browser;
  try {
    await run(browser);
  } finally {
    globalThis.window = previous;
  }
}

test("connect scrubs and retains the launch token and authenticates same-origin requests", async () => {
  await withWindow("http://127.0.0.1:8123/app?singleserve_token=launch-secret&view=one#section", async (browser) => {
    const calls = [];
    const heartbeats = [];
    const session = await connect({
      fetch: async (input, init) => {
        calls.push({ input: String(input), init });
        return ok();
      },
      heartbeatIntervalMS: 60_000,
      onHeartbeat: (heartbeat) => heartbeats.push(heartbeat),
    });

    assert.equal(session.token, "launch-secret");
    assert.equal(browser.sessionStorage.getItem(TOKEN_STORAGE_KEY), "launch-secret");
    assert.deepEqual(browser.history.replacements, [
      { state: { preserved: true }, unused: "", url: "/app?view=one#section" },
    ]);
    assert.match(session.tabID, /^[A-Za-z0-9_-]{22}$/);
    assert.equal(calls[0].input, "/_singleserve/tabs/heartbeat");
    assert.equal(calls[0].init.headers.get(TOKEN_HEADER), "launch-secret");
    assert.equal(calls[0].init.headers.get(TAB_HEADER), session.tabID);
    assert.equal(calls[0].init.credentials, "same-origin");
    assert.equal(heartbeats.length, 1);
    assert.equal(heartbeats[0].ok, true);
    assert.equal(heartbeats[0].failures, 0);
    assert.equal(heartbeats[0].tabID, session.tabID);
    assert.ok(heartbeats[0].checkedAt instanceof Date);

    await session.fetch("/api/items", { headers: { Accept: "application/json" } });
    assert.equal(calls[1].input, "/api/items");
    assert.equal(calls[1].init.headers.get("Accept"), "application/json");
    assert.equal(calls[1].init.headers.get(TOKEN_HEADER), "launch-secret");
    session.stop();
    assert.equal((browser.listeners.get("pagehide") ?? []).length, 0);
  });
});

test("stored tokens authenticate reloads without exposing a query token", async () => {
  await withWindow("http://localhost:9000/", async (browser) => {
    browser.sessionStorage.setItem(TOKEN_STORAGE_KEY, "stored-secret");
    let token;
    const session = await connect({
      fetch: async (_input, init) => {
        token = init.headers.get(TOKEN_HEADER);
        return ok();
      },
      heartbeatIntervalMS: 60_000,
    });
    assert.equal(session.token, "stored-secret");
    assert.equal(token, "stored-secret");
    assert.deepEqual(browser.history.replacements, []);
    session.stop();
  });
});

test("authenticated fetch refuses to send credentials cross-origin", async () => {
  await withWindow("http://localhost:9000/?singleserve_token=secret", async () => {
    let calls = 0;
    const session = await connect({
      fetch: async () => {
        calls += 1;
        return ok();
      },
      heartbeatIntervalMS: 60_000,
    });
    assert.throws(
      () => session.fetch("https://example.com/collect"),
      (error) => error instanceof SingleserveError && /another origin/.test(error.message),
    );
    assert.equal(calls, 1, "only the initial heartbeat should reach fetch");
    session.stop();
  });
});

test("shutdown denials surface stable status and application code", async () => {
  await withWindow("http://localhost:9000/?singleserve_token=secret", async () => {
    const session = await connect({
      fetch: async (input) => {
        if (String(input).endsWith("/shutdown")) {
          return ok({ ok: false, error: { code: "unsaved_changes", message: "Save first" } }, 409);
        }
        return ok();
      },
      heartbeatIntervalMS: 60_000,
    });
    await assert.rejects(
      session.requestShutdown(),
      (error) => error instanceof SingleserveError && error.status === 409 && error.code === "unsaved_changes" && error.message === "Save first",
    );
    session.stop();
  });
});

test("heartbeat failure callback fires once at the configured threshold", async () => {
  await withWindow("http://localhost:9000/?singleserve_token=secret", async () => {
    let callbacks = 0;
    let resolveUnavailable;
    const unavailable = new Promise((resolve) => {
      resolveUnavailable = resolve;
    });
    const session = await connect({
      fetch: async () => ok({ ok: false }, 503),
      heartbeatIntervalMS: 1,
      heartbeatTimeoutMS: 50,
      failureThreshold: 2,
      onHeartbeat() {
        throw new Error("presentation callback failure");
      },
      onServerUnavailable(details) {
        callbacks += 1;
        resolveUnavailable(details);
      },
    });
    const details = await Promise.race([
      unavailable,
      new Promise((_, reject) => setTimeout(() => reject(new Error("callback timeout")), 500)),
    ]);
    assert.equal(details.failures, 2);
    await new Promise((resolve) => setTimeout(resolve, 10));
    assert.equal(callbacks, 1);
    session.stop();
  });
});

test("a successful heartbeat resets consecutive failures", async () => {
  await withWindow("http://localhost:9000/?singleserve_token=secret", async () => {
    const health = [false, true, false, false];
    const observedFailures = [];
    let calls = 0;
    let resolveUnavailable;
    const unavailable = new Promise((resolve) => {
      resolveUnavailable = resolve;
    });
    const session = await connect({
      fetch: async () => {
        const healthy = health[calls] ?? false;
        calls += 1;
        return healthy ? ok() : ok({ ok: false }, 503);
      },
      heartbeatIntervalMS: 1,
      heartbeatTimeoutMS: 50,
      failureThreshold: 2,
      onHeartbeat: ({ failures }) => observedFailures.push(failures),
      onServerUnavailable: resolveUnavailable,
    });
    const details = await Promise.race([
      unavailable,
      new Promise((_, reject) => setTimeout(() => reject(new Error("callback timeout")), 500)),
    ]);
    assert.equal(details.failures, 2);
    assert.equal(calls, 4, "the success should reset the first failure");
    assert.deepEqual(observedFailures, [1, 0, 1, 2]);
    session.stop();
  });
});

test("pagehide emits an idempotent keepalive disconnect", async () => {
  await withWindow("http://localhost:9000/?singleserve_token=secret", async (browser) => {
    const calls = [];
    const session = await connect({
      fetch: async (input, init) => {
        calls.push({ input: String(input), init });
        return ok();
      },
      heartbeatIntervalMS: 60_000,
    });
    browser.dispatch("pagehide");
    await new Promise((resolve) => setTimeout(resolve, 0));
    const disconnect = calls.find((call) => call.input.endsWith("/tabs/disconnect"));
    assert.ok(disconnect);
    assert.equal(disconnect.init.keepalive, true);
    session.stop();
    session.stop();
  });
});
