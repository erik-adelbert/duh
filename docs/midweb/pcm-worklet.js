class MIDPCMPlayer extends AudioWorkletProcessor {
  constructor(options) {
    super();
    this.step = (options.processorOptions.sourceRate || 49716) / sampleRate;
    this.chunks = [];
    this.byteOffset = 0;
    this.phase = 0;
    this.frameA = null;
    this.frameB = null;

    this.port.onmessage = event => {
      const bytes = event.data;
      if (bytes instanceof Uint8Array && bytes.length % 4 === 0) {
        this.chunks.push(bytes);
      }
    };
  }

  nextFrame() {
    while (this.chunks.length > 0) {
      const chunk = this.chunks[0];
      if (this.byteOffset + 3 >= chunk.length) {
        this.chunks.shift();
        this.byteOffset = 0;
        continue;
      }

      const offset = this.byteOffset;
      const leftWord = chunk[offset] | (chunk[offset + 1] << 8);
      const rightWord = chunk[offset + 2] | (chunk[offset + 3] << 8);
      this.byteOffset += 4;
      return [
        (leftWord < 0x8000 ? leftWord : leftWord - 0x10000) / 32768,
        (rightWord < 0x8000 ? rightWord : rightWord - 0x10000) / 32768
      ];
    }
    return null;
  }

  process(inputs, outputs) {
    const output = outputs[0];
    const left = output[0];
    const right = output[1];

    for (let i = 0; i < left.length; i++) {
      if (this.frameA === null) this.frameA = this.nextFrame();
      if (this.frameA === null) {
        left[i] = 0;
        right[i] = 0;
        continue;
      }
      if (this.frameB === null) this.frameB = this.nextFrame();
      if (this.frameB === null) {
        left[i] = 0;
        right[i] = 0;
        continue;
      }

      left[i] = this.frameA[0] + (this.frameB[0] - this.frameA[0]) * this.phase;
      right[i] = this.frameA[1] + (this.frameB[1] - this.frameA[1]) * this.phase;

      this.phase += this.step;
      while (this.phase >= 1) {
        this.frameA = this.frameB;
        this.frameB = this.nextFrame();
        this.phase -= 1;
        if (this.frameB === null) break;
      }
    }

    return true;
  }
}

registerProcessor("mid-pcm-player", MIDPCMPlayer);