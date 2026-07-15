import assert from "node:assert/strict";
import test from "node:test";

import { BOOTSTRAP_HEADER, BOOTSTRAP_PATH, establishBrowserSession, installBrowserBootstrap } from "./bootstrap.js";

function fakeWindow(hash = "#bootstrap-secret") {
  const replacements = [];
  const navigations = [];
  const listeners = new Map();
  const browser = {
    location: {
      hash,
      pathname: BOOTSTRAP_PATH,
      search: "",
      replace: (target) => navigations.push(target),
    },
    history: {
      state: { preserved: true },
      replaceState: (state, unused, target) => replacements.push({ state, unused, target }),
    },
    addEventListener: (name, listener) => listeners.set(name, listener),
    removeEventListener: (name, listener) => {
      if (listeners.get(name) === listener) listeners.delete(name);
    },
  };
  Object.defineProperty(browser, "sessionStorage", {
    get() {
      throw new Error("sessionStorage must not be accessed");
    },
  });
  Object.defineProperty(browser, "localStorage", {
    get() {
      throw new Error("localStorage must not be accessed");
    },
  });
  return { browser, replacements, navigations, listeners };
}

test("bootstrap scrubs the fragment before exchange and replaces the page", async () => {
  const { browser, replacements, navigations } = fakeWindow();
  const calls = [];
  await establishBrowserSession({
    window: browser,
    fetch: async (input, init) => {
      calls.push({ input, init });
      assert.deepEqual(replacements, [
        { state: { preserved: true }, unused: "", target: BOOTSTRAP_PATH },
      ]);
      return new Response(null, { status: 200 });
    },
  });
  assert.equal(calls.length, 1);
  assert.equal(calls[0].input, BOOTSTRAP_PATH);
  assert.equal(calls[0].init.headers[BOOTSTRAP_HEADER], "bootstrap-secret");
  assert.equal(calls[0].init.credentials, "same-origin");
  assert.deepEqual(navigations, ["/"]);
  assert.equal(JSON.stringify({ replacements, navigations }).includes("bootstrap-secret"), false);
});

test("missing and rejected capabilities fail without navigation or token echo", async () => {
  const missing = fakeWindow("");
  await assert.rejects(
    establishBrowserSession({ window: missing.browser, fetch: async () => new Response(null, { status: 200 }) }),
    /invalid or expired/,
  );
  assert.deepEqual(missing.navigations, []);

  const rejected = fakeWindow("#private-value");
  await assert.rejects(
    establishBrowserSession({ window: rejected.browser, fetch: async () => new Response(null, { status: 401 }) }),
    (error) => /invalid or expired/.test(error.message) && !error.message.includes("private-value"),
  );
  assert.deepEqual(rejected.navigations, []);
});

test("a failed history scrub replaces the capability-bearing page without exchange", async () => {
  const { browser, navigations } = fakeWindow("#private-value");
  browser.history.replaceState = () => {
    throw new Error("history unavailable");
  };
  let fetches = 0;
  await assert.rejects(
    establishBrowserSession({
      window: browser,
      fetch: async () => {
        fetches += 1;
        return new Response(null, { status: 200 });
      },
    }),
    (error) => /invalid or expired/.test(error.message) && !error.message.includes("private-value"),
  );
  assert.equal(fetches, 0);
  assert.deepEqual(navigations, [BOOTSTRAP_PATH]);
});

test("a renewed fragment retries bootstrap in the same failed page", async () => {
  const { browser, listeners, navigations, replacements } = fakeWindow("#expired-value");
  const calls = [];
  const status = { textContent: "Starting" };
  const cleanup = installBrowserBootstrap({
    window: browser,
    document: { getElementById: () => status },
    fetch: async (input, init) => {
      calls.push({ input, init });
      return new Response(null, { status: calls.length === 1 ? 401 : 200 });
    },
  });
  await new Promise(setImmediate);
  assert.equal(status.textContent.includes("invalid or expired"), true);
  assert.equal(calls.length, 1);

  browser.location.hash = "#renewed-value";
  listeners.get("hashchange")();
  await new Promise(setImmediate);
  assert.equal(calls.length, 2);
  assert.equal(calls[1].input, BOOTSTRAP_PATH);
  assert.equal(calls[1].init.headers[BOOTSTRAP_HEADER], "renewed-value");
  assert.deepEqual(navigations, ["/"]);
  assert.equal(JSON.stringify({ replacements, navigations }).includes("renewed-value"), false);

  cleanup();
  assert.equal(listeners.has("hashchange"), false);
});
