"use strict";

const fileInput = document.getElementById("file");
const choose = document.getElementById("choose");
const fileName = document.getElementById("file-name");
const terminal = document.getElementById("terminal-frame");
const noLoop = document.getElementById("noloop");
const sequence = document.getElementById("sequence");
const play = document.getElementById("play");
const resume = document.getElementById("resume");
const stop = document.getElementById("stop");
const status = document.getElementById("status");
const warnings = document.getElementById("warnings");
const sampleRate = 48000;
const maxFileSize = 10 * 1024 * 1024;
const chunkBytes = 8192;
const maxQueued = 4;
let ready = false;
let session = null;

function updateControls() {
  play.disabled = !ready || !fileInput.files.length || session !== null;
  stop.disabled = session === null;
  resume.disabled = session === null || !session.context || session.context.state === "running";
  fileInput.disabled = session !== null;
  choose.disabled = session !== null;
  noLoop.disabled = session !== null;
  sequence.disabled = session !== null;
}

async function endSession(current, message) {
  if (session !== current) return;
  session = null;
  clearInterval(current.display);
  if (current.node) current.node.disconnect();
  const error = current.opened ? globalThis.closeModule() : null;
  if (current.context) {
    try {
      await current.context.close();
    } catch (closeError) {
      message += ` Audio close failed: ${closeError.message}`;
    }
  }
  status.textContent = error ? `${message} ${error}` : message;
  updateControls();
}

function pump(current) {
  if (session !== current || current.finishing || current.context.state !== "running") return;
  try {
    while (current.queued < maxQueued) {
      const buffer = new Uint8Array(chunkBytes);
      const result = globalThis.readModule(buffer);
      if (result.error) throw new Error(result.error);
      if (result.bytes) {
        const data = result.bytes === buffer.length ? buffer.buffer : buffer.buffer.slice(0, result.bytes);
        current.queued++;
        current.node.port.postMessage(data, [data]);
      }
      if (result.eof) {
        current.finishing = true;
        current.node.port.postMessage("finish");
        break;
      }
      if (!result.bytes) throw new Error("Decoder returned no audio without ending.");
    }
  } catch (error) {
    void endSession(current, `Playback failed: ${error.message}`);
  }
}

globalThis.moduleReady = () => {
  ready = true;
  status.textContent = "Choose a module.";
  updateControls();
};

choose.addEventListener("click", () => fileInput.click());
fileInput.addEventListener("change", () => {
  fileName.textContent = fileInput.files.length ? fileInput.files[0].name : "No file selected";
  updateControls();
});
stop.addEventListener("click", () => {
  if (session) void endSession(session, "Stopped.");
});
resume.addEventListener("click", async () => {
  const current = session;
  if (!current || !current.context) return;
  try {
    await current.context.resume();
    if (current.context.state !== "running") throw new Error("Audio is still suspended.");
  } catch (error) {
    if (session === current) status.textContent = `Audio resume failed: ${error.message}`;
  }
});

play.addEventListener("click", async () => {
  if (!ready || session || !fileInput.files.length) return;
  const file = fileInput.files[0];
  if (!file.size || file.size > maxFileSize) {
    status.textContent = "Module files must be between 1 byte and 10 MiB.";
    return;
  }
  if (!sequence.validity.valid || sequence.value === "") {
    status.textContent = "Choose a non-negative MPTM sequence number.";
    return;
  }
  const current = { context: null, node: null, opened: false, queued: 0, frames: 0, finishing: false };
  session = current;
  warnings.textContent = "";
  status.textContent = "Loading module...";
  updateControls();
  try {
    if (navigator.audioSession) navigator.audioSession.type = "playback";
    current.context = new AudioContext({ sampleRate });
    await current.context.resume();
    if (session !== current) return;
    if (current.context.state !== "running") throw new Error("Audio is suspended.");
    if (current.context.sampleRate !== sampleRate) throw new Error("This browser cannot provide 48 kHz audio.");
    await current.context.audioWorklet.addModule("pcm-worklet.js");
    if (session !== current) return;
    const data = await file.arrayBuffer();
    if (session !== current) return;
    const result = globalThis.openModule(new Uint8Array(data), noLoop.checked, Number(sequence.value));
    if (result.error) throw new Error(result.error);
    current.opened = true;
    warnings.textContent = result.warnings.join("\n");
    current.display = setInterval(() => {
      if (session === current) {
        terminal.contentWindow.postMessage({
          type: "screen",
          screen: globalThis.renderModule(current.frames / sampleRate)
        }, "*");
      }
    }, 1000 / 30);
    current.node = new AudioWorkletNode(current.context, "module-pcm-player", {
      numberOfInputs: 0, numberOfOutputs: 1, outputChannelCount: [2]
    });
    current.node.port.onmessage = event => {
      if (session !== current) return;
      if (event.data.type === "consumed") {
        current.queued--;
        current.frames += event.data.frames;
        pump(current);
      } else if (event.data.type === "ended") {
        void endSession(current, "Playback finished.");
      }
    };
    current.node.onprocessorerror = () => void endSession(current, "Audio processor failed.");
    current.context.onstatechange = () => {
      if (session !== current) return;
      if (current.context.state === "running") {
        status.textContent = "Playing.";
        pump(current);
      } else if (current.context.state === "suspended" || current.context.state === "interrupted") {
        status.textContent = "Audio suspended. Use Resume audio to continue.";
      }
      updateControls();
    };
    current.node.connect(current.context.destination);
    status.textContent = "Playing.";
    pump(current);
  } catch (error) {
    await endSession(current, `Playback failed: ${error.message}`);
  }
});

const go = new Go();
WebAssembly.instantiateStreaming(fetch("modweb.wasm"), go.importObject)
  .then(({ instance }) => go.run(instance))
  .catch(error => {
    status.textContent = `Player failed to load: ${error.message}`;
  });
