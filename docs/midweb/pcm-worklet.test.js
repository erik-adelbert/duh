"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

const load = name => fs.readFileSync(path.join(__dirname, name), "utf8");
const PCMBufferPool = vm.runInNewContext(`${load("pcm-buffer-pool.js")}\nPCMBufferPool`, {
  ArrayBuffer, Uint8Array
});

function createPlayer(receive) {
  let Player;
  class AudioWorkletProcessor {
    constructor() {
      this.port = {
        postMessage: (buffer, transfer) => receive(structuredClone(buffer, { transfer }))
      };
    }
  }
  vm.runInNewContext(load("pcm-worklet.js"), {
    AudioWorkletProcessor, Uint8Array, console,
    registerProcessor: (_name, implementation) => { Player = implementation; }
  });
  return new Player();
}

function send(player, bytes) {
  const buffer = bytes.buffer;
  player.port.onmessage({ data: structuredClone(bytes, { transfer: [buffer] }) });
  assert.equal(buffer.byteLength, 0);
}

function render(player, frames) {
  const left = new Float32Array(frames);
  const right = new Float32Array(frames);
  assert.equal(player.process([], [[left, right]]), true);
  return [Array.from(left), Array.from(right)];
}

test("worklet returns buffers only after consuming all frames", () => {
  const pool = new PCMBufferPool(4096, 8);
  const returned = [];
  const player = createPlayer(buffer => {
    returned.push(buffer);
    pool.recycle(buffer);
  });
  const bytes = pool.acquire(8);
  bytes.set([0, 64, 0, 192, 255, 127, 0, 128]);
  send(player, bytes);

  assert.deepEqual(render(player, 1), [[0.5], [-0.5]]);
  assert.equal(returned.length, 0);
  assert.deepEqual(render(player, 2), [[32767 / 32768, 0], [-1, 0]]);
  assert.equal(returned.length, 1);
  assert.equal(returned[0].byteLength, 4096);

  const reused = pool.acquire(4);
  assert.equal(reused.buffer, returned[0]);
  reused.set([0, 32, 0, 224]);
  send(player, reused);
  assert.deepEqual(render(player, 2), [[0.25, 0], [-0.25, 0]]);
  assert.equal(returned.length, 2);
});

test("pool bounds retained buffers and handles unusually large chunks", () => {
  const pool = new PCMBufferPool(4096, 2);
  const player = createPlayer(buffer => pool.recycle(buffer));
  const chunks = [pool.acquire(4096), pool.acquire(4096), pool.acquire(4096)];
  chunks.forEach(bytes => send(player, bytes));
  render(player, 3 * 1024);
  assert.equal(pool.buffers.length, 2);

  const large = pool.acquire(8192);
  assert.equal(large.byteLength, 8192);
  assert.equal(pool.buffers.length, 2);
  pool.acquire(4096);
  pool.acquire(4096);
  send(player, large);
  render(player, 2048);
  assert.equal(pool.buffers.length, 0);
});

test("worklet plays queued chunks in order across a render boundary", () => {
  const pool = new PCMBufferPool(4096, 8);
  const player = createPlayer(buffer => pool.recycle(buffer));
  const first = pool.acquire(8);
  const second = pool.acquire(4);
  first.set([0, 64, 0, 192, 0, 32, 0, 224]);
  second.set([0, 16, 0, 240]);
  send(player, first);
  send(player, second);

  assert.deepEqual(render(player, 1), [[0.5], [-0.5]]);
  assert.deepEqual(render(player, 3), [[0.25, 0.125, 0], [-0.25, -0.125, 0]]);
  assert.equal(pool.buffers.length, 2);
});
