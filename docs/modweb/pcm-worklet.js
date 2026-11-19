class ModulePCMPlayer extends AudioWorkletProcessor {
  constructor() {
    super();
    this.queue = [];
    this.offset = 0;
    this.finishing = false;
    this.port.onmessage = event => {
      if (event.data === "finish") {
        this.finishing = true;
      } else if (event.data instanceof ArrayBuffer) {
        this.queue.push(new DataView(event.data));
      }
    };
  }

  process(_inputs, outputs) {
    const [left, right] = outputs[0];
    for (let i = 0; i < left.length; i++) {
      if (this.queue.length === 0) break;
      const pcm = this.queue[0];
      left[i] = pcm.getFloat32(this.offset, true);
      right[i] = pcm.getFloat32(this.offset + 4, true);
      this.offset += 8;
      if (this.offset === pcm.byteLength) {
        this.queue.shift();
        this.offset = 0;
        this.port.postMessage({ type: "consumed", frames: pcm.byteLength / 8 });
      }
    }
    if (this.finishing && this.queue.length === 0) {
      this.port.postMessage({ type: "ended" });
      return false;
    }
    return true;
  }
}

registerProcessor("module-pcm-player", ModulePCMPlayer);
