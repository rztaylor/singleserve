export const BOOTSTRAP_HEADER = "X-Singleserve-Bootstrap";
export const BOOTSTRAP_PATH = "/_singleserve/bootstrap";

export async function establishBrowserSession(options = {}) {
  const browser = options.window ?? globalThis.window;
  const fetchImpl = options.fetch ?? globalThis.fetch;
  if (!browser?.location || !browser?.history || typeof fetchImpl !== "function") {
    throw new Error("Secure browser bootstrap is unavailable");
  }

  const token = String(browser.location.hash || "").replace(/^#/, "");
  const clean = `${browser.location.pathname}${browser.location.search}` || BOOTSTRAP_PATH;
  try {
    browser.history.replaceState(browser.history.state, "", clean);
  } catch {
    browser.location.replace(BOOTSTRAP_PATH);
    throw new Error("The browser bootstrap link is invalid or expired");
  }
  if (!token) throw new Error("The browser bootstrap link is invalid or expired");

  const response = await fetchImpl(BOOTSTRAP_PATH, {
    method: "POST",
    cache: "no-store",
    credentials: "same-origin",
    headers: { [BOOTSTRAP_HEADER]: token },
  });
  if (!response.ok) throw new Error("The browser bootstrap link is invalid or expired");
  browser.location.replace("/");
}

function showFailure(document = globalThis.document) {
  const status = document?.getElementById?.("status");
  if (status) status.textContent = "This browser link is invalid or expired. Return to the application and request a new launch.";
}

export function installBrowserBootstrap(options = {}) {
  const browser = options.window ?? globalThis.window;
  const document = options.document ?? browser?.document ?? globalThis.document;
  const start = () => {
    void establishBrowserSession(options).catch(() => showFailure(document));
  };
  browser?.addEventListener?.("hashchange", start);
  start();
  return () => browser?.removeEventListener?.("hashchange", start);
}

if (globalThis.window?.document) {
  installBrowserBootstrap();
}
