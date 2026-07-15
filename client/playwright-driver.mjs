import http from "node:http";
import process from "node:process";

import { chromium, firefox, webkit } from "playwright";

const portArgument = process.argv.find((argument) => argument.startsWith("--port="));
const port = Number.parseInt(portArgument?.slice("--port=".length) ?? "", 10);
if (!Number.isInteger(port) || port < 1 || port > 65535) {
  throw new Error("playwright driver requires --port=<1-65535>");
}

let browser;
let context;
let sessionID;
let currentHandle;
let nextHandle = 1;
const pages = new Map();

function response(res, status, value) {
  const body = JSON.stringify({ value: value === undefined ? null : value });
  res.writeHead(status, {
    "Cache-Control": "no-store",
    "Content-Type": "application/json; charset=utf-8",
  });
  res.end(body);
}

function failure(res, error) {
  response(res, 500, {
    error: "unknown error",
    message: String(error?.message ?? error),
    stacktrace: String(error?.stack ?? ""),
  });
}

async function payload(req) {
  const chunks = [];
  for await (const chunk of req) {
    chunks.push(chunk);
  }
  if (chunks.length === 0) {
    return {};
  }
  return JSON.parse(Buffer.concat(chunks).toString("utf8"));
}

function activePage() {
  const page = pages.get(currentHandle);
  if (!page) {
    throw new Error("no active browser page");
  }
  return page;
}

function registerPage(page) {
  const handle = `page-${nextHandle}`;
  nextHandle += 1;
  pages.set(handle, page);
  page.once("close", () => pages.delete(handle));
  return handle;
}

function requestedBrowserName(capabilities) {
  const name = capabilities?.alwaysMatch?.browserName ?? "chromium";
  if (name === "chrome") {
    return "chromium";
  }
  return name;
}

function browserType(name) {
  switch (name) {
    case "chromium":
      return chromium;
    case "firefox":
      return firefox;
    case "webkit":
      return webkit;
    default:
      throw new Error(`unsupported Playwright browser: ${name}`);
  }
}

async function closeSession() {
  if (context) {
    await context.close();
  }
  if (browser) {
    await browser.close();
  }
  browser = undefined;
  context = undefined;
  sessionID = undefined;
  currentHandle = undefined;
  pages.clear();
}

async function execute(page, script, args) {
  return page.evaluate(
    ({ source, values }) => {
      const fn = new Function(source);
      return fn(...values);
    },
    { source: script, values: args },
  );
}

async function executeAsync(page, script, args) {
  return page.evaluate(
    ({ source, values }) =>
      new Promise((resolve, reject) => {
        try {
          const fn = new Function(source);
          fn(...values, resolve);
        } catch (error) {
          reject(error);
        }
      }),
    { source: script, values: args },
  );
}

const server = http.createServer(async (req, res) => {
  try {
    const url = new URL(req.url, `http://${req.headers.host}`);
    const path = url.pathname;

    if (req.method === "GET" && path === "/status") {
      response(res, 200, { ready: true, message: "Playwright adapter ready" });
      return;
    }

    if (req.method === "POST" && path === "/session") {
      if (sessionID) {
        throw new Error("only one Playwright browser session is supported");
      }
      const body = await payload(req);
      const name = requestedBrowserName(body.capabilities);
      browser = await browserType(name).launch({ headless: true });
      context = await browser.newContext();
      const page = await context.newPage();
      currentHandle = registerPage(page);
      sessionID = "playwright";
      response(res, 200, { sessionId: sessionID, capabilities: { browserName: name } });
      return;
    }

    if (!sessionID || !path.startsWith(`/session/${sessionID}`)) {
      response(res, 404, { error: "invalid session id", message: "unknown browser session" });
      return;
    }

    const suffix = path.slice(`/session/${sessionID}`.length);
    if (req.method === "DELETE" && suffix === "") {
      await closeSession();
      response(res, 200, null);
      return;
    }
    if (req.method === "POST" && suffix === "/url") {
      const body = await payload(req);
      await activePage().goto(body.url, { waitUntil: "load" });
      response(res, 200, null);
      return;
    }
    if (req.method === "POST" && suffix === "/refresh") {
      await activePage().reload({ waitUntil: "load" });
      response(res, 200, null);
      return;
    }
    if (req.method === "POST" && suffix === "/execute/sync") {
      const body = await payload(req);
      response(res, 200, await execute(activePage(), body.script, body.args ?? []));
      return;
    }
    if (req.method === "POST" && suffix === "/execute/async") {
      const body = await payload(req);
      response(res, 200, await executeAsync(activePage(), body.script, body.args ?? []));
      return;
    }
    if (req.method === "GET" && suffix === "/window") {
      response(res, 200, currentHandle);
      return;
    }
    if (req.method === "POST" && suffix === "/window/new") {
      const handle = registerPage(await context.newPage());
      response(res, 200, { handle, type: "tab" });
      return;
    }
    if (req.method === "POST" && suffix === "/window") {
      const body = await payload(req);
      if (!pages.has(body.handle)) {
        throw new Error(`unknown page handle: ${body.handle}`);
      }
      currentHandle = body.handle;
      response(res, 200, null);
      return;
    }
    if (req.method === "DELETE" && suffix === "/window") {
      const page = activePage();
      const closedHandle = currentHandle;
      await page.close({ runBeforeUnload: false });
      pages.delete(closedHandle);
      currentHandle = pages.keys().next().value;
      response(res, 200, [...pages.keys()]);
      return;
    }
    if (req.method === "DELETE" && suffix === "/cookie") {
      await context.clearCookies();
      response(res, 200, null);
      return;
    }
    if (req.method === "GET" && suffix === "/cookie") {
      response(res, 200, await context.cookies());
      return;
    }

    response(res, 404, { error: "unknown command", message: `${req.method} ${suffix}` });
  } catch (error) {
    failure(res, error);
  }
});

async function shutdown() {
  server.close();
  await closeSession();
  process.exit(0);
}

process.once("SIGINT", shutdown);
process.once("SIGTERM", shutdown);

server.listen(port, "127.0.0.1");
