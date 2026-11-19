"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

function playerContext(steps) {
  const elements = new Map();
  const timers = [];
  const context = {
    document: {
      getElementById(id) {
        if (!elements.has(id)) elements.set(id, { addEventListener() {}, textContent: "", files: [] });
        return elements.get(id);
      }
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
  vm.runInContext("session = {}", context);
  return { context, elements, timers };
}

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
