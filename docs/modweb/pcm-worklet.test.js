"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

test("worklet plays float stereo chunks, reports consumption and drains", () => {
  let Player;
  const messages = [];
  class AudioWorkletProcessor {
    constructor() {
      this.port = { postMessage: message => messages.push(message) };
    }
  }
  vm.runInNewContext(fs.readFileSync(path.join(__dirname, "pcm-worklet.js"), "utf8"), {
    AudioWorkletProcessor, ArrayBuffer, DataView,
    registerProcessor: (_name, implementation) => { Player = implementation; }
  });
  const player = new Player();
  const buffer = new ArrayBuffer(16);
  const data = new DataView(buffer);
  [0.25, -0.5, 0.75, -1].forEach((value, i) => data.setFloat32(i * 4, value, true));
  player.port.onmessage({ data: buffer });
  player.port.onmessage({ data: "finish" });
  const left = new Float32Array(3);
  const right = new Float32Array(3);
  assert.equal(player.process([], [[left, right]]), false);
  assert.deepEqual([...left], [0.25, 0.75, 0]);
  assert.deepEqual([...right], [-0.5, -1, 0]);
  assert.equal(messages.length, 2);
  assert.equal(messages[0].type, "consumed");
  assert.equal(messages[0].frames, 2);
  assert.equal(messages[1].type, "ended");
});
