"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

function playerContext(steps) {
  const elements = new Map();
  const timers = [];
  const listeners = new Map();
  const wakeEvents = [];
  const terminalMessages = [];
  const context = {
    document: {
      visibilityState: "visible",
      addEventListener() {},
      getElementById(id) {
        if (!elements.has(id)) elements.set(id, {
          addEventListener(type, callback) { listeners.set(`${id}:${type}`, callback); },
          textContent: "", files: [],
          contentWindow: { postMessage(message) { terminalMessages.push(message); } }
        });
        return elements.get(id);
      }
    },
    window: { addEventListener() {} },
    navigator: {},
    PlaybackWakeLock: class {
      start() { wakeEvents.push("start"); }
      stop() { wakeEvents.push("stop"); }
      visibilityChanged() {}
    },
    setTimeout(callback) { timers.push(callback); return timers.length; },
    clearTimeout() {},
    Go: class { constructor() { this.importObject = {}; } },
    fetch() {},
    WebAssembly: { instantiateStreaming() { return new Promise(() => {}); } },
    stepModuleDuration() { return steps.shift(); }
  };
  vm.createContext(context);
  vm.runInContext(fs.readFileSync(path.join(__dirname, "player.js"), "utf8"), context);
  vm.runInContext('session = { context: { state: "running" }, paused: false }', context);
  return { context, elements, timers, listeners, wakeEvents, terminalMessages };
}

test("ending a session clears the terminal without clearing a later session", async () => {
  const { context, terminalMessages, elements } = playerContext([]);
  context.clearInterval = () => {};
  vm.runInContext("session.context.close = async () => {}", context);
  const current = vm.runInContext("session", context);
  await vm.runInContext('endSession(session, "Playback finished.")', context);
  assert.equal(terminalMessages.length, 1);
  assert.equal(terminalMessages[0].type, "clear-screen");
  assert.equal(elements.get("status").textContent, "Playback finished.");

  context.oldSession = current;
  vm.runInContext('session = { context: { state: "running" }, paused: false }', context);
  await vm.runInContext('endSession(oldSession, "Stopped.")', context);
  assert.equal(terminalMessages.length, 1);
});

test("duration is calculated in cancellable batches", () => {
  const { context, timers } = playerContext([{ done: false }, { done: true }]);
  vm.runInContext("calculateDuration(session)", context);
  assert.equal(timers.length, 1);
  timers.shift()();
  assert.equal(timers.length, 0);
});

test("duration failures preserve other warnings", () => {
  const { context, elements, timers } = playerContext([{ error: "complexity limit exceeded" }]);
  elements.get("warnings").textContent = "Existing warning";
  vm.runInContext("calculateDuration(session)", context);
  assert.equal(elements.get("warnings").textContent,
    "Existing warning\nDuration unavailable: complexity limit exceeded");
  assert.equal(timers.length, 0);
});

test("stale duration callbacks do not run after stopping", () => {
  const { context, timers } = playerContext([{ done: false }]);
  vm.runInContext("calculateDuration(session); session = null", context);
  timers.shift()();
  assert.equal(timers.length, 0);
});

test("pause and resume preserve queued audio and stop decoder work", async () => {
  const { context, elements, listeners, wakeEvents, timers } = playerContext([{ done: true }]);
  let reads = 0;
  const messages = [];
  context.readModule = () => { reads++; return { bytes: 8, eof: true }; };
  context.current = {
    context: {
      state: "running",
      async suspend() { this.state = "suspended"; },
      async resume() { this.state = "running"; }
    },
    node: { port: { postMessage(message) { messages.push(message); } } },
    paused: false, controlPending: false, queued: 4, frames: 123, durationDone: false
  };
  vm.runInContext("session = current; updateControls()", context);
  await listeners.get("pause:click")();
  assert.equal(elements.get("pause").textContent, "Resume");
  assert.equal(elements.get("status").textContent, "Paused.");
  assert.deepEqual(wakeEvents, ["stop"]);
  vm.runInContext("pump(session); calculateDuration(session)", context);
  assert.equal(reads, 0);
  assert.equal(timers.length, 0);
  assert.equal(context.current.frames, 123);
  assert.equal(context.current.queued, 4);
  await listeners.get("pause:click")();
  assert.equal(elements.get("pause").textContent, "Pause");
  assert.deepEqual(wakeEvents, ["stop", "start"]);
  assert.equal(context.current.queued, 4);
  assert.equal(messages.length, 0);
  assert.equal(timers.length, 1);
});

test("pause and resume failures keep the appropriate state and report errors", async () => {
  const { context, elements, listeners, wakeEvents } = playerContext([]);
  context.current = {
    context: {
      state: "running",
      async suspend() { throw new Error("suspend denied"); },
      async resume() { throw new Error("resume denied"); }
    },
    node: { port: { postMessage() {} } },
    paused: false, queued: 4, durationDone: true
  };
  vm.runInContext("session = current", context);
  await listeners.get("pause:click")();
  assert.equal(context.current.paused, false);
  assert.match(elements.get("status").textContent, /suspend denied/);
  assert.equal(elements.get("pause").textContent, "Pause");
  context.current.paused = true;
  context.current.context.state = "suspended";
  await listeners.get("pause:click")();
  assert.equal(context.current.paused, true);
  assert.match(elements.get("status").textContent, /resume denied/);
  assert.equal(elements.get("pause").textContent, "Resume");
  assert.deepEqual(wakeEvents, []);
});

test("stopping while a pause is pending cannot reactivate playback", async () => {
  const { context, elements, listeners, wakeEvents } = playerContext([]);
  let resolveSuspend;
  context.current = {
    context: {
      state: "running",
      suspend() { return new Promise(resolve => { resolveSuspend = resolve; }); },
      async close() { this.state = "closed"; }
    },
    node: { disconnect() {}, port: {} },
    paused: false, queued: 4, durationDone: true
  };
  context.clearInterval = () => {};
  vm.runInContext("session = current", context);
  const pending = listeners.get("pause:click")();
  listeners.get("stop:click")();
  resolveSuspend();
  await pending;
  await Promise.resolve();
  assert.equal(vm.runInContext("session", context), null);
  assert.equal(elements.get("pause").disabled, true);
  assert.deepEqual(wakeEvents, ["stop"]);
});

test("playback enables pause and resumes after an audio interruption", async () => {
  const { context, elements, listeners, wakeEvents } = playerContext([{ done: true }]);
  context.AudioContext = class {
    constructor() {
      this.state = "suspended";
      this.sampleRate = 48000;
      this.audioWorklet = { async addModule() {} };
      this.destination = {};
    }
    async resume() { this.state = "running"; }
  };
  context.AudioWorkletNode = class {
    constructor() { this.port = { postMessage() {} }; }
    connect() {}
  };
  context.openModule = () => ({ warnings: [] });
  context.readModule = buffer => ({ bytes: buffer.length, eof: false });
  context.setInterval = () => 1;
  elements.get("file").files = [{ size: 8, async arrayBuffer() { return new ArrayBuffer(8); } }];
  elements.get("sequence").validity = { valid: true };
  elements.get("sequence").value = "0";
  vm.runInContext("ready = true; session = null", context);
  await listeners.get("play:click")();
  assert.equal(elements.get("pause").disabled, false);
  assert.equal(elements.get("pause").textContent, "Pause");
  vm.runInContext('session.context.state = "interrupted"; session.context.onstatechange()', context);
  assert.equal(elements.get("pause").textContent, "Resume");
  assert.match(elements.get("status").textContent, /Audio suspended/);
  await listeners.get("pause:click")();
  assert.equal(elements.get("pause").textContent, "Pause");
  assert.equal(elements.get("status").textContent, "Playing.");
  assert.deepEqual(wakeEvents, ["start", "stop", "start"]);
});
