"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");

test("standalone wake lock implementation matches midweb", () => {
  const vgm = fs.readFileSync(path.join(__dirname, "screen-wake-lock.js"), "utf8");
  const mid = fs.readFileSync(path.join(__dirname, "../midweb/screen-wake-lock.js"), "utf8");
  assert.equal(vgm.slice(vgm.indexOf("\n") + 1), mid);
});

test("standalone terminal implementation matches midweb", () => {
  const vgm = fs.readFileSync(path.join(__dirname, "playback-terminal.js"), "utf8");
  const mid = fs.readFileSync(path.join(__dirname, "../midweb/playback-terminal.js"), "utf8");
  assert.equal(vgm.slice(vgm.indexOf("\n") + 1), mid);
});
