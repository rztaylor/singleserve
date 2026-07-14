import {
  DEFAULT_FAILURE_THRESHOLD,
  DEFAULT_HEARTBEAT_INTERVAL_MS,
  DEFAULT_HEARTBEAT_TIMEOUT_MS,
  connect,
} from "/_singleserve/client.js";

const app = document.querySelector("#app");
const connection = document.querySelector("#connection");
const pulse = document.querySelector("#pulse");
const heartbeatState = document.querySelector("#heartbeat-state");
const heartbeatDetail = document.querySelector("#heartbeat-detail");
const nextHeartbeat = document.querySelector("#next-heartbeat");
const detectionWindow = document.querySelector("#detection-window");
const closeWindow = document.querySelector("#close-window");
const closeDetail = document.querySelector("#close-detail");
const requestObserved = document.querySelector("#request-observed");
const healthResult = document.querySelector("#health-result");
const failureDots = [...document.querySelectorAll("#failure-dots span")];
const eventList = document.querySelector("#event-list");
const eventCount = document.querySelector("#event-count");
const healthButton = document.querySelector("#health");
const shutdownButton = document.querySelector("#shutdown");
const terminal = document.querySelector("#terminal");
const terminalHeading = document.querySelector("#terminal-heading");
const terminalDetail = document.querySelector("#terminal-detail");
const closeTabButton = document.querySelector("#close-tab");

const heartbeatTimeoutMS = Number(app.dataset.heartbeatTimeoutMs);
const disconnectGraceMS = Number(app.dataset.disconnectGraceMs);
const checkIntervalMS = Number(app.dataset.checkIntervalMs);
const detectionMS = DEFAULT_FAILURE_THRESHOLD * (DEFAULT_HEARTBEAT_INTERVAL_MS + DEFAULT_HEARTBEAT_TIMEOUT_MS);
const normalCloseMS = disconnectGraceMS + checkIntervalMS;
const fallbackCloseMS = heartbeatTimeoutMS + disconnectGraceMS + checkIntervalMS;

detectionWindow.textContent = `Within ~${seconds(detectionMS)}`;
closeWindow.textContent = `~${seconds(normalCloseMS)} normally`;
closeDetail.textContent = `If disconnect is lost, heartbeat expiry stops it within ~${seconds(fallbackCloseMS)}.`;

let session;
let heartbeatCount = 0;
let nextHeartbeatAt = 0;
let stopped = false;

const countdownTimer = window.setInterval(() => {
  if (!nextHeartbeatAt || stopped) return;
  const remaining = Math.max(0, nextHeartbeatAt - Date.now());
  nextHeartbeat.textContent = `${Math.ceil(remaining / 1000)} s`;
}, 250);

function seconds(milliseconds) {
  return `${Math.ceil(milliseconds / 1000)} s`;
}

function clockTime(value = new Date()) {
  return new Intl.DateTimeFormat([], { hour: "2-digit", minute: "2-digit", second: "2-digit" }).format(value);
}

function addEvent(summary, detail) {
  const item = document.createElement("li");
  const time = document.createElement("time");
  const text = document.createElement("span");
  time.dateTime = new Date().toISOString();
  time.textContent = clockTime();
  text.textContent = detail ? `${summary} — ${detail}` : summary;
  item.append(time, text);
  eventList.prepend(item);
  while (eventList.children.length > 5) eventList.lastElementChild.remove();
  const count = eventList.children.length;
  eventCount.textContent = `${count} ${count === 1 ? "event" : "events"}`;
}

function showHeartbeat(result) {
  heartbeatCount += 1;
  nextHeartbeatAt = result.checkedAt.getTime() + DEFAULT_HEARTBEAT_INTERVAL_MS;
  requestObserved.textContent = clockTime(result.checkedAt);
  pulse.classList.remove("active");
  requestAnimationFrame(() => pulse.classList.add("active"));
  heartbeatState.textContent = result.ok
    ? `Heartbeat #${heartbeatCount} succeeded at ${clockTime(result.checkedAt)}`
    : `Heartbeat #${heartbeatCount} failed at ${clockTime(result.checkedAt)}`;
  heartbeatDetail.textContent = `Authenticated POST every ${seconds(DEFAULT_HEARTBEAT_INTERVAL_MS)} · ${result.failures} consecutive failures`;
  failureDots.forEach((dot, index) => dot.classList.toggle("failed", index < result.failures));
  document.querySelector("#failure-dots").setAttribute("aria-label", `${result.failures} of ${DEFAULT_FAILURE_THRESHOLD} heartbeat failures`);
  addEvent(result.ok ? "Heartbeat succeeded" : "Heartbeat failed", `${result.failures}/${DEFAULT_FAILURE_THRESHOLD} failures`);
}

function setConnected() {
  connection.textContent = "Connected";
  connection.className = "status";
  healthButton.disabled = false;
  shutdownButton.disabled = false;
  addEvent("Browser session connected", `tab ${session.tabID.slice(0, 8)}…`);
}

function enterTerminal(heading, detail) {
  if (stopped) return;
  stopped = true;
  session?.stop();
  window.clearInterval(countdownTimer);
  connection.textContent = "Stopped";
  connection.className = "status stopped";
  heartbeatState.textContent = "Heartbeat monitoring stopped";
  heartbeatDetail.textContent = detail;
  nextHeartbeat.textContent = "—";
  healthButton.disabled = true;
  shutdownButton.disabled = true;
  terminalHeading.textContent = heading;
  terminalDetail.textContent = detail;
  terminal.hidden = false;
  document.title = "Backend stopped · Singleserve";
  addEvent(heading, detail);
  window.close();
}

session = await connect({
  onHeartbeat: showHeartbeat,
  onServerUnavailable({ failures }) {
    enterTerminal("Backend connection lost", `The frontend observed ${failures} consecutive failed heartbeats.`);
  },
});
setConnected();

healthButton.addEventListener("click", async () => {
  healthButton.disabled = true;
  healthResult.textContent = "Health request in progress…";
  const started = new Date();
  const healthy = await session.health();
  const detail = `${healthy ? "Healthy" : "Unavailable"} at ${clockTime(started)}`;
  requestObserved.textContent = clockTime(started);
  healthResult.textContent = detail;
  addEvent("Manual health check", detail);
  if (!stopped) healthButton.disabled = false;
});

shutdownButton.addEventListener("click", async () => {
  shutdownButton.disabled = true;
  addEvent("Shutdown requested", "Waiting for the backend to accept it");
  try {
    await session.requestShutdown();
    enterTerminal("Backend shutdown accepted", "The server is draining and this tab is no longer needed.");
  } catch (error) {
    addEvent("Shutdown denied", error.message);
    healthResult.textContent = error.message;
    shutdownButton.disabled = false;
  }
});

closeTabButton.addEventListener("click", () => window.close());
