export const TAB_HEADER = "X-Singleserve-Tab";
export const CONTROL_PATH = "/_singleserve/";

export const DEFAULT_HEARTBEAT_INTERVAL_MS = 5_000;
export const DEFAULT_HEARTBEAT_TIMEOUT_MS = 2_000;
export const DEFAULT_FAILURE_THRESHOLD = 3;

export class SingleserveError extends Error {
  constructor(message, status = 0, code = "") {
    super(message);
    this.name = "SingleserveError";
    this.status = status;
    this.code = code;
  }
}

export async function connect(options = {}) {
  const browser = globalThis.window;
  if (!browser?.location || !browser?.history) {
    throw new SingleserveError("Singleserve requires a browser window");
  }

  const fetchImpl = options.fetch ?? globalThis.fetch;
  if (typeof fetchImpl !== "function") {
    throw new SingleserveError("Fetch is unavailable");
  }

  const heartbeatIntervalMS = positiveNumber(options.heartbeatIntervalMS, DEFAULT_HEARTBEAT_INTERVAL_MS);
  const heartbeatTimeoutMS = positiveNumber(options.heartbeatTimeoutMS, DEFAULT_HEARTBEAT_TIMEOUT_MS);
  const failureThreshold = positiveNumber(options.failureThreshold, DEFAULT_FAILURE_THRESHOLD);
  const onServerUnavailable = typeof options.onServerUnavailable === "function" ? options.onServerUnavailable : () => {};
  const onHeartbeat = typeof options.onHeartbeat === "function" ? options.onHeartbeat : () => {};
  const tabID = createTabID();
  const origin = new URL(browser.location.href).origin;

  let stopped = false;
  let timer;
  let heartbeatAbort;
  let failures = 0;
  let unavailableNotified = false;

  const clearTimer = () => {
    if (timer !== undefined) {
      browser.clearTimeout(timer);
      timer = undefined;
    }
  };

  const authenticatedFetch = (input, init = {}) => {
    assertSameOrigin(input, origin);
    const inputHeaders = typeof Request !== "undefined" && input instanceof Request ? input.headers : undefined;
    const headers = new Headers(init.headers ?? inputHeaders);
    headers.delete("X-Singleserve-Token");
    headers.set(TAB_HEADER, tabID);
    return fetchImpl(input, { ...init, headers, credentials: "same-origin" });
  };

  const health = async (signal) => {
    try {
      const response = await authenticatedFetch(`${CONTROL_PATH}health`, { cache: "no-store", signal });
      const payload = await response.json().catch(() => ({}));
      return response.ok && payload.ok === true;
    } catch {
      return false;
    }
  };

  const sendHeartbeat = async () => {
    const controller = new AbortController();
    heartbeatAbort = controller;
    const timeout = browser.setTimeout(() => controller.abort(), heartbeatTimeoutMS);
    try {
      const response = await authenticatedFetch(`${CONTROL_PATH}tabs/heartbeat`, {
        method: "POST",
        cache: "no-store",
        signal: controller.signal,
      });
      const payload = await response.json().catch(() => ({}));
      return response.ok && payload.ok === true;
    } catch {
      return false;
    } finally {
      browser.clearTimeout(timeout);
      if (heartbeatAbort === controller) heartbeatAbort = undefined;
    }
  };

  const schedule = () => {
    if (stopped || unavailableNotified) return;
    clearTimer();
    timer = browser.setTimeout(tick, heartbeatIntervalMS);
  };

  const tick = async () => {
    timer = undefined;
    const healthy = await sendHeartbeat();
    if (stopped) return;
    failures = healthy ? 0 : failures + 1;
    try {
      onHeartbeat({ ok: healthy, failures, tabID, checkedAt: new Date() });
    } catch {
      // Consumer telemetry must not break lifecycle scheduling.
    }
    if (failures >= failureThreshold) {
      unavailableNotified = true;
      onServerUnavailable({ failures, tabID });
      return;
    }
    schedule();
  };

  const disconnect = async () => {
    try {
      const response = await authenticatedFetch(`${CONTROL_PATH}tabs/disconnect`, {
        method: "POST",
        cache: "no-store",
        keepalive: true,
      });
      return response.ok;
    } catch {
      return false;
    }
  };

  const handlePageHide = () => {
    clearTimer();
    void disconnect();
  };

  const handlePageShow = () => {
    if (stopped) return;
    failures = 0;
    unavailableNotified = false;
    void tick();
  };

  browser.addEventListener?.("pagehide", handlePageHide);
  browser.addEventListener?.("pageshow", handlePageShow);

  await tick();

  return {
    tabID,
    fetch: authenticatedFetch,
    health,
    disconnect,
    async requestShutdown() {
      const response = await authenticatedFetch(`${CONTROL_PATH}shutdown`, { method: "POST", cache: "no-store" });
      const payload = await response.json().catch(() => ({}));
      if (!response.ok) {
        throw new SingleserveError(payload.error?.message || "Could not shut down local server", response.status, payload.error?.code || "");
      }
      return payload;
    },
    stop() {
      if (stopped) return;
      stopped = true;
      clearTimer();
      heartbeatAbort?.abort();
      browser.removeEventListener?.("pagehide", handlePageHide);
      browser.removeEventListener?.("pageshow", handlePageShow);
    },
  };
}

function createTabID() {
  const bytes = new Uint8Array(16);
  globalThis.crypto.getRandomValues(bytes);
  let binary = "";
  for (const value of bytes) binary += String.fromCharCode(value);
  return globalThis.btoa(binary).replaceAll("+", "-").replaceAll("/", "_").replace(/=+$/, "");
}

function assertSameOrigin(input, origin) {
  const raw = typeof Request !== "undefined" && input instanceof Request ? input.url : String(input);
  const target = new URL(raw, origin);
  if (target.origin !== origin) {
    throw new SingleserveError("Singleserve will not send authentication to another origin");
  }
}

function positiveNumber(value, fallback) {
  return Number.isFinite(value) && value > 0 ? value : fallback;
}
