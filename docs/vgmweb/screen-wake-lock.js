// Keep in sync with docs/midweb/screen-wake-lock.js; both sites can be served independently.
class PlaybackWakeLock {
  constructor(wakeLock, isVisible, report) {
    this.wakeLock = wakeLock;
    this.isVisible = isVisible;
    this.report = report;
    this.active = false;
    this.generation = 0;
    this.sentinel = null;
  }

  start() {
    this.active = true;
    this.acquire();
  }

  async acquire() {
    if (!this.active || this.sentinel) return;
    if (!this.isVisible()) {
      this.report("Screen wake lock paused while the page is hidden.");
      return;
    }
    const generation = ++this.generation;
    if (!this.wakeLock) {
      this.report("Screen wake lock unavailable; the screen may sleep during playback.");
      return;
    }

    this.report("Requesting screen wake lock...");
    let sentinel;
    try {
      sentinel = await this.wakeLock.request("screen");
    } catch (error) {
      if (generation === this.generation && this.active) {
        this.report(`Screen wake lock failed: ${error.message || String(error)}. The screen may sleep.`);
      }
      return;
    }

    if (generation !== this.generation || !this.active || !this.isVisible()) {
      await this.release(sentinel);
      return;
    }
    this.sentinel = sentinel;
    sentinel.addEventListener("release", () => {
      if (this.sentinel !== sentinel) return;
      this.sentinel = null;
      if (this.active && this.isVisible()) {
        this.report("Screen wake lock released; the screen may sleep.");
      }
    });
    if (sentinel.released) {
      this.sentinel = null;
      this.report("Screen wake lock released; the screen may sleep.");
      return;
    }
    this.report("");
  }

  async release(sentinel) {
    try {
      await sentinel.release();
    } catch (error) {
      console.warn("Could not release screen wake lock:", error);
    }
  }

  visibilityChanged() {
    if (!this.isVisible()) {
      this.generation++;
      const sentinel = this.sentinel;
      this.sentinel = null;
      if (sentinel) this.release(sentinel);
      if (this.active) this.report("Screen wake lock paused while the page is hidden.");
    } else if (this.active) {
      this.acquire();
    }
  }

  stop() {
    this.active = false;
    this.generation++;
    const sentinel = this.sentinel;
    this.sentinel = null;
    if (sentinel) this.release(sentinel);
    this.report("");
  }
}
