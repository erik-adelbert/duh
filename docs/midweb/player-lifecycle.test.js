"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

function playerContext(file) {
  const html = fs.readFileSync(path.join(__dirname, file), "utf8");
  const script = html.match(/<script>\s*([\s\S]*?)<\/script>/)[1];
  const elements = new Map();
  const handlers = {};
  const messages = [];
  const wakeEvents = [];
  let stops = 0;
  const frame = {
    postMessage(message) { messages.push(message); },
    async pausePlayback() {},
    async resumePlayback() {},
    stopPlayback() { stops++; }
  };
  const context = {
    document: {
      visibilityState: "visible",
      addEventListener() {},
      createElement() { return {}; },
      getElementById(id) {
        if (!elements.has(id)) elements.set(id, {
          disabled: false, textContent: "", dataset: {}, style: {}, files: [],
          value: "0", contentWindow: frame,
          addEventListener(type, handler) { handlers[`${id}:${type}`] = handler; },
          replaceChildren() {}, append() {}, click() {}
        });
        return elements.get(id);
      }
    },
    window: { addEventListener(type, handler) { handlers[`window:${type}`] = handler; } },
    navigator: {},
    PlaybackWakeLock: class {
      start() { wakeEvents.push("start"); }
      stop() { wakeEvents.push("stop"); }
      visibilityChanged() {}
    },
    console: { error() {} }
  };
  vm.createContext(context);
  vm.runInContext(script, context);
  const event = data => handlers["window:message"]({ source: frame, data });
  const invoke = (id, type) => (handlers[`${id}:${type}`] || elements.get(id)[`on${type}`])();
  return { context, elements, messages, wakeEvents, event, invoke, stops: () => stops };
}

for (const [file, input] of [["index.html", "midi-file"], ["../vgmweb/index.html", "file"]]) {
  test(`${file} stops paused playback, preserves errors, and allows replay`, async () => {
    const player = playerContext(file);
    if (input === "midi-file") player.context.populateBanks(["bank"]);
    else player.event({ type: "player-ready" });
    player.elements.get(input).files = [{ name: "test", size: 4, async arrayBuffer() { return new ArrayBuffer(4); } }];
    player.invoke(input, "change");
    await player.invoke("play", "click");
    player.event({ type: "playback-started" });
    assert.equal(player.elements.get("stop").disabled, false);
    player.event({ type: "audio-blocked" });
    assert.equal(player.wakeEvents.at(-1), "stop");
    player.event({ type: "audio-unlocked" });
    assert.equal(player.wakeEvents.at(-1), "start");
    await player.invoke("pause", "click");
    assert.equal(player.elements.get("pause").textContent, "Resume");
    player.invoke("stop", "click");
    assert.equal(player.stops(), 1);
    const wakeCount = player.wakeEvents.length;
    player.event({ type: "audio-unlocked" });
    assert.equal(player.wakeEvents.length, wakeCount);
    player.event({ type: "playback-ended" });
    assert.equal(player.elements.get("status").textContent, "Stopped.");
    assert.equal(player.elements.get("play").disabled, false);

    await player.invoke("play", "click");
    assert.equal(player.messages.filter(message => message.type === "play").length, 2);
    player.event({ type: "error", message: "decode failed" });
    player.event({ type: "playback-ended" });
    assert.equal(player.elements.get("status").textContent, "decode failed");
    assert.equal(player.elements.get("play").disabled, false);
  });
}
