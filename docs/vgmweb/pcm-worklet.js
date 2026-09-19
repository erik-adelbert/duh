class PCMPlayerProcessor extends AudioWorkletProcessor {
  constructor() {
    super();

    /*
     * Each entry is a Uint8Array containing:
     *
     *   signed 16-bit little-endian stereo PCM
     *
     * Four bytes per frame:
     *
     *   byte 0: left low
     *   byte 1: left high
     *   byte 2: right low
     *   byte 3: right high
     */
    this.queue = [];

    this.offset = 0;

    this.port.onmessage = event => {
      const bytes = event.data;

      if (!(bytes instanceof Uint8Array)) {
        return;
      }

      this.queue.push(bytes);
    };

    console.log("PCM player worklet constructed");
  }


  process(inputs, outputs) {
    const output = outputs[0];

    const left = output[0];
    const right = output[1];

    for (let i = 0; i < left.length; i++) {
      /*
       * No PCM available.
       *
       * Output silence rather than garbage.
       */
      if (this.queue.length === 0) {
        left[i] = 0;
        right[i] = 0;
        continue;
      }

      const pcm = this.queue[0];

      /*
       * Stereo s16le:
       *
       *   L = bytes 0,1
       *   R = bytes 2,3
       */
      const byteOffset = this.offset;

      const leftWord =
        pcm[byteOffset] |
        (pcm[byteOffset + 1] << 8);

      const rightWord =
        pcm[byteOffset + 2] |
        (pcm[byteOffset + 3] << 8);


      /*
       * Convert unsigned 16-bit representation
       * into signed 16-bit.
       */
      const leftSample =
        leftWord < 0x8000
          ? leftWord
          : leftWord - 0x10000;

      const rightSample =
        rightWord < 0x8000
          ? rightWord
          : rightWord - 0x10000;


      /*
       * Web Audio output buffers are Float32Array,
       * so this is the ONLY place where the PCM
       * becomes float.
       */
      left[i] =
        leftSample / 32768;

      right[i] =
        rightSample / 32768;


      /*
       * Advance one stereo frame.
       */
      this.offset += 4;


      /*
       * Finished this Go/JS PCM chunk.
       */
      if (this.offset >= pcm.length) {
        this.queue.shift();
        this.offset = 0;
      }
    }

    return true;
  }
}


registerProcessor(
  "pcm-player",
  PCMPlayerProcessor
);