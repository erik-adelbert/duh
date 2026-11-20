"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

function terminalContext(file) {
  const html = fs.readFileSync(path.join(__dirname, file), "utf8");
  const script = html.match(/<script>\s*([\s\S]*?)<\/script>/)[1];
  const writes = [];
  const addons = [];
  const listeners = {};
  const posts = [];
  const parent = { postMessage(message) { posts.push(message); } };
  const context = {
    parent,
    document: {
      getElementById() { return {}; },
      removeEventListener() {}
    },
    Terminal: class {
      constructor() {
        this.element = { querySelector() { return { getBoundingClientRect() { return { height: 1 }; } }; } };
      }
      open() {}
      loadAddon(addon) { addons.push(addon); }
      write(text) { writes.push(text); }
    },
    WebglAddon: { WebglAddon: class { dispose() { this.disposed = true; } } },
    ResizeObserver: class { observe() {} },
    PCMBufferPool: class {},
    Go: class { constructor() { this.importObject = {}; } },
    fetch() {},
    WebAssembly: { instantiateStreaming() { return new Promise(() => {}); } },
    console: { error() {}, log() {}, warn() {} },
    addEventListener(type, callback) { listeners[type] = callback; }
  };
  context.window = context;
  vm.createContext(context);
  vm.runInContext(fs.readFileSync(path.join(__dirname, "playback-terminal.js"), "utf8"), context);
  vm.runInContext(script, context);
  return { context, parent, writes, addons, listeners, posts };
}

for (const file of ["mid.html", "../vgmweb/vgm.html"]) {
  test(`${file} clears successful playback and preserves errors`, () => {
    const { context, writes, addons } = terminalContext(file);
    context.playbackEnded();
    assert.equal(addons[0].disposed, true);
    assert.equal(writes.at(-1), "\x1b[0m\x1b[2J\x1b[H");

    vm.runInContext("playbackTerminal.start()", context);
    assert.equal(addons.length, 2);
    context.renderError("playback error");
    context.playbackEnded();
    assert.equal(addons[1].disposed, true);
    assert.match(writes.at(-1), /playback error/);
  });

  test(`${file} stop disconnects queued audio even while paused`, () => {
    const { context } = terminalContext(file);
    let disconnected = false;
    context.testNode = { disconnect() { disconnected = true; } };
    vm.runInContext("audioNode = testNode; paused = true", context);
    context.stopPlayback();
    assert.equal(disconnected, true);
    assert.equal(context.playbackStopped(), true);
    assert.equal(context.playbackPaused(), false);
    assert.equal(vm.runInContext("audioNode", context), null);
  });

  test(`${file} treats an interrupted audio context as paused`, () => {
    const { context } = terminalContext(file);
    vm.runInContext('audioContext = { state: "interrupted" }', context);
    assert.equal(context.playbackPaused(), true);
    vm.runInContext('audioContext.state = "running"', context);
    assert.equal(context.playbackPaused(), false);
  });
}

test("VGM terminal reports readiness when requested after initialization", () => {
  const { context, parent, listeners, posts } = terminalContext("../vgmweb/vgm.html");
  context.playerReady();
  posts.length = 0;
  listeners.message({ source: parent, data: { type: "request-ready" } });
  assert.equal(posts.length, 1);
  assert.equal(posts[0].type, "player-ready");
});

test("module terminal clears on completion and re-enables WebGL on the next frame", () => {
  const { parent, writes, addons, listeners } = terminalContext("../modweb/mod.html");
  listeners.message({ source: parent, data: { type: "clear-screen" } });
  assert.equal(addons[0].disposed, true);
  assert.equal(writes.at(-1), "\x1b[0m\x1b[2J\x1b[H");
  listeners.message({ source: parent, data: { type: "screen", screen: "next frame" } });
  assert.equal(addons.length, 2);
  assert.equal(writes.at(-1), "next frame");
});
