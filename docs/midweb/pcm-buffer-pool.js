class PCMBufferPool {
  constructor(chunkBytes, maxBuffers) {
    this.chunkBytes = chunkBytes;
    this.maxBuffers = maxBuffers;
    this.buffers = [];
  }

  acquire(length) {
    const buffer = length <= this.chunkBytes && this.buffers.length
      ? this.buffers.pop()
      : new ArrayBuffer(Math.max(length, this.chunkBytes));
    return new Uint8Array(buffer, 0, length);
  }

  recycle(buffer) {
    if (buffer instanceof ArrayBuffer &&
        buffer.byteLength === this.chunkBytes &&
        this.buffers.length < this.maxBuffers) {
      this.buffers.push(buffer);
    }
  }
}
