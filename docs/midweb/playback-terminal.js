class PlaybackTerminal {
  constructor(term) {
    this.term = term;
    this.webglAddon = null;
    this.webglAttempted = false;
  }

  start() {
    if (this.webglAttempted) return;
    this.webglAttempted = true;
    try {
      const addon = new WebglAddon.WebglAddon();
      this.term.loadAddon(addon);
      this.webglAddon = addon;
    } catch (error) {
      console.warn("WebGL terminal unavailable; using the default renderer:", error);
    }
  }

  finish(clear) {
    // Removing the WebGL canvas also removes stale background cells.
    if (this.webglAddon) {
      this.webglAddon.dispose();
      this.webglAddon = null;
    }
    this.webglAttempted = false;
    if (clear) this.term.write("\x1b[0m\x1b[2J\x1b[H");
  }
}
