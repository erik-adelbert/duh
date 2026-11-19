"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

const PlaybackWakeLock = vm.runInNewContext(
  `${fs.readFileSync(path.join(__dirname, "screen-wake-lock.js"), "utf8")}\nPlaybackWakeLock`,
  { console }
);

function makeSentinel() {
  let onRelease;
  return {
    released: false,
    addEventListener: (_type, callback) => { onRelease = callback; },
    async release() {
      this.released = true;
      onRelease?.();
    }
  };
}

const flush = () => new Promise(resolve => setImmediate(resolve));

test("holds lock during playback and reacquires after visibility changes", async () => {
  let visible = true;
  const sentinels = [];
  const messages = [];
  const controller = new PlaybackWakeLock({
    async request(type) {
      assert.equal(type, "screen");
      const sentinel = makeSentinel();
      sentinels.push(sentinel);
      return sentinel;
    }
  }, () => visible, message => messages.push(message));
  controller.start();
  await flush();
  assert.equal(controller.sentinel, sentinels[0]);
  assert.equal(messages.at(-1), "");
  visible = false;
  controller.visibilityChanged();
  assert.equal(sentinels[0].released, true);
  assert.match(messages.at(-1), /hidden/);
  visible = true;
  controller.visibilityChanged();
  await flush();
  assert.equal(controller.sentinel, sentinels[1]);
  controller.stop();
  assert.equal(sentinels[1].released, true);
  assert.equal(messages.at(-1), "");
});

test("releases a request that resolves after playback stops or the page hides", async () => {
  let resolve;
  let visible = true;
  const messages = [];
  const controller = new PlaybackWakeLock({
    request: () => new Promise(done => { resolve = done; })
  }, () => visible, message => messages.push(message));
  controller.start();
  controller.stop();
  const first = makeSentinel();
  resolve(first);
  await flush();
  assert.equal(first.released, true);
  assert.equal(messages.at(-1), "");

  controller.start();
  visible = false;
  controller.visibilityChanged();
  const second = makeSentinel();
  resolve(second);
  await flush();
  assert.equal(second.released, true);
  assert.match(messages.at(-1), /hidden/);
});

test("reports unsupported, denied and externally released locks", async () => {
  const messages = [];
  const unsupported = new PlaybackWakeLock(undefined, () => true, message => messages.push(message));
  unsupported.start();
  assert.match(messages.at(-1), /unavailable/);
  const denied = new PlaybackWakeLock({
    request: async () => { throw new Error("not allowed"); }
  }, () => true, message => messages.push(message));
  denied.start();
  await flush();
  assert.match(messages.at(-1), /not allowed/);
  const sentinel = makeSentinel();
  const external = new PlaybackWakeLock({ request: async () => sentinel }, () => true, message => messages.push(message));
  external.start();
  await flush();
  await sentinel.release();
  assert.equal(external.sentinel, null);
  assert.match(messages.at(-1), /released/);
});

test("reports pending requests and playback started while hidden", async () => {
  let visible = false;
  let resolve;
  const messages = [];
  const controller = new PlaybackWakeLock({
    request: () => new Promise(done => { resolve = done; })
  }, () => visible, message => messages.push(message));
  controller.start();
  assert.match(messages.at(-1), /hidden/);
  visible = true;
  controller.visibilityChanged();
  assert.match(messages.at(-1), /Requesting/);
  resolve(makeSentinel());
  await flush();
  assert.equal(messages.at(-1), "");
  controller.stop();
});
