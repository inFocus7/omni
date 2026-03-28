(() => {
  // Shared state for widget picker — written by openPicker(), read by single confirm handler
  let widgetPickerSelection = { widget: null, size: null, filter: null };

  // ── Toast Notifications ──────────────────────────────────────
  function showToast(message, options = {}) {
    const {
      type = "info", // 'error', 'warning', 'success', 'info'
      title = null,
      duration = 5000,
      closeable = true,
    } = options;

    const container = document.getElementById("toast-container");
    if (!container) return;

    const toast = document.createElement("div");
    toast.className = `toast toast--${type}`;

    const icons = {
      error:
        '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>',
      warning:
        '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>',
      success:
        '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 11-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>',
      info: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg>',
    };

    toast.innerHTML = `
      <div class="toast-icon toast-icon--${type}">${icons[type]}</div>
      <div class="toast-content">
        ${title ? `<div class="toast-title">${escapeHtml(title)}</div>` : ""}
        <div class="toast-message">${escapeHtml(message)}</div>
      </div>
      ${closeable ? '<button class="toast-close" aria-label="Dismiss">&times;</button>' : ""}
    `;

    const closeBtn = toast.querySelector(".toast-close");
    if (closeBtn) {
      closeBtn.addEventListener("click", () => removeToast(toast));
    }

    container.appendChild(toast);

    if (duration > 0) {
      setTimeout(() => removeToast(toast), duration);
    }

    return toast;
  }

  function removeToast(toast) {
    if (!toast || toast.classList.contains("toast--removing")) return;
    toast.classList.add("toast--removing");
    setTimeout(() => toast.remove(), 150);
  }

  function escapeHtml(text) {
    const div = document.createElement("div");
    div.textContent = text;
    return div.innerHTML;
  }

  // ── Generic confirmation dialog ───────────────────────────────────────────
  {
    const overlay = document.getElementById("confirm-overlay");
    const msgEl = document.getElementById("confirm-msg");
    const yesBtn = document.getElementById("confirm-yes");
    const noBtn = document.getElementById("confirm-no");
    let _onYes = null,
      _onNo = null;

    window.showConfirm = function (
      msg,
      onYes,
      onNo,
      { yesLabel = "Confirm", noLabel = "Cancel" } = {},
    ) {
      if (msgEl) msgEl.textContent = msg;
      if (yesBtn) yesBtn.textContent = yesLabel;
      if (noBtn) noBtn.textContent = noLabel;
      _onYes = onYes;
      _onNo = onNo;
      if (overlay) overlay.hidden = false;
    };

    window.dismissConfirm = function () {
      if (overlay) overlay.hidden = true;
      _onYes = null;
      _onNo = null;
    };

    if (yesBtn)
      yesBtn.addEventListener("click", () => {
        const cb = _onYes;
        dismissConfirm();
        cb?.();
      });
    if (noBtn)
      noBtn.addEventListener("click", () => {
        const cb = _onNo;
        dismissConfirm();
        cb?.();
      });
  }

  // Plugin metadata
  const PLUGIN_META = {
    github: {
      name: "GitHub",
      icon: '<svg class="plugin-card-icon" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z"/></svg>',
      badgeIcon:
        '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z"/></svg>',
    },
    spacer: {
      name: "Layout",
      icon: '<svg class="plugin-card-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/></svg>',
    },
    ascii: {
      name: "ASCII",
      icon: '<svg class="plugin-card-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="18" height="18" rx="2"/><path d="M7 8h2l1 4 1-4h2M15 8v8M7 16h3"/></svg>',
      badgeIcon:
        '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="18" height="18" rx="2"/><path d="M7 8h2l1 4 1-4h2M15 8v8M7 16h3"/></svg>',
    },
  };

  // Helper to get the active filter value
  function getActiveFilter() {
    const activeLink = document.querySelector(".filters a.active");
    if (!activeLink) return "7d";
    const url = new URL(activeLink.href);
    return url.searchParams.get("filter") || "7d";
  }

  // Check for widget errors on page load and show toasts
  document.addEventListener("DOMContentLoaded", () => {
    const errorWidgets = document.querySelectorAll(".widget-error[data-error]");
    errorWidgets.forEach((widget) => {
      const widgetId = widget.dataset.widgetId || "unknown";
      const error = widget.dataset.error || "Unknown error";
      showToast(error, {
        type: "error",
        title: `Widget "${widgetId}" failed to load`,
        duration: 8000,
      });
    });
  });

  const header = document.querySelector(".site-header");
  if (header) {
    let lastY = window.scrollY;
    window.addEventListener(
      "scroll",
      () => {
        const y = window.scrollY;
        if (y > lastY && y > 80) {
          header.classList.add("site-header--hidden");
        } else {
          header.classList.remove("site-header--hidden");
        }
        lastY = y;
      },
      { passive: true },
    );
  }

  // ── Client-side navigation for filter links ────────────
  function initNav() {
    const main = document.querySelector("main.plugins");
    if (!main) return;

    main.addEventListener("click", (e) => {
      const link = e.target.closest(".filters a");
      if (!link) return;
      e.preventDefault();

      const url = link.href;
      navigateTo(url);
    });
  }

  let navController = null;

  function navigateTo(url) {
    const main = document.querySelector("main.plugins");
    if (!main) return;

    // Abort any in-flight navigation
    if (navController) navController.abort();
    navController = new AbortController();

    // Immediately update active filter
    const filters = main.querySelector(".filters");
    if (filters) {
      filters.querySelectorAll("a").forEach((a) => {
        a.classList.toggle("active", a.href === url);
      });
    }

    main.classList.add("plugins--loading");

    fetch(url, { signal: navController.signal })
      .then((res) => {
        if (!res.ok) throw new Error(res.statusText);
        return res.text();
      })
      .then((html) => {
        const doc = new DOMParser().parseFromString(html, "text/html");
        const newMain = doc.querySelector("main.plugins");
        if (!newMain) throw new Error("no main found");

        // Swap content and re-init components
        main.innerHTML = newMain.innerHTML;
        main.classList.remove("plugins--loading");
        main
          .querySelectorAll(".plugin, .widget")
          .forEach((el) => (el.style.animation = "none"));
        initSortables();
        initWidgetGrid();
        initBadges();
        initAsciiAnimations();

        // Update URL without reload
        history.pushState(null, "", url);

        // Update page title if changed
        const newTitle = doc.querySelector("title");
        if (newTitle) document.title = newTitle.textContent;
      })
      .catch((err) => {
        if (err.name === "AbortError") return;
        // On failure, fall back to normal navigation
        main.classList.remove("plugins--loading");
        window.location.href = url;
      });
  }

  // Handle back/forward
  window.addEventListener("popstate", () => {
    navigateTo(window.location.href);
  });

  function initSortables() {
    document.querySelectorAll("table[data-sortable]").forEach(initSortable);
  }

  // ── Badge system ─────────────────────────────────────────
  function initBadges() {
    document
      .querySelectorAll(".widget-badge[data-plugin-id]")
      .forEach((badge) => {
        const meta = PLUGIN_META[badge.dataset.pluginId];
        if (meta?.badgeIcon) {
          badge.title = meta.name;
          badge.innerHTML = meta.badgeIcon;
        } else {
          badge.remove();
        }
      });
  }

  // ── ASCII animation controller ───────────────────────────

  // ── ICG Canvas Renderer ─────────────────────────────────
  // Renders ASCII animations on a <canvas> element using fillText.
  // Wire format: { palette: string[], cols, rows, frames: [{chars, colors}] }
  // where colors is base64-encoded Uint8Array indexing into palette.

  let _monoCharRatio = null;
  function getMonoCharRatio(ctx) {
    if (_monoCharRatio !== null) return _monoCharRatio;
    const prev = ctx.font;
    ctx.font = "100px 'JetBrains Mono', monospace";
    _monoCharRatio = ctx.measureText("M").width / 100;
    ctx.font = prev;
    return _monoCharRatio;
  }
  document.fonts.ready.then(() => { _monoCharRatio = null; });

  class AsciiCanvasRenderer {
    constructor(container, data, fps) {
      this.container = container;
      this.palette = data.palette;
      this.cols = data.cols;
      this.rows = data.rows;
      this.fps = fps;
      this.interval = 1000 / fps;
      this.currentFrame = 0;
      this.lastFrameTime = 0;
      this.paused = false;
      this.rafId = null;
      this.indicator = container.querySelector(".ascii-pause-indicator");

      // Decode base64 colors for each frame into Uint8Array
      this.frames = data.frames.map((f) => ({
        chars: f.chars,
        colors: Uint8Array.from(atob(f.colors), (c) => c.charCodeAt(0)),
      }));

      // Create canvas element
      this.canvas = document.createElement("canvas");
      this.canvas.className = "ascii-render-canvas";
      this.canvas.style.width = "100%";
      this.canvas.style.height = "100%";
      // Insert canvas before pause indicator
      container.classList.remove("ascii-loading");
      if (this.indicator) {
        container.insertBefore(this.canvas, this.indicator);
      } else {
        container.appendChild(this.canvas);
      }

      this.ctx = this.canvas.getContext("2d");
      this._charW = 0;
      this._charH = 0;
      this._fontSize = 0;

      // Resize canvas and measure font on container resize
      this._ro = new ResizeObserver(() => this._resize());
      this._ro.observe(container);
      this._resize();
      this._renderFrame(0);

      // If fonts weren't ready yet, re-measure and re-render once they load
      if (document.fonts.status !== "loaded") {
        document.fonts.ready.then(() => this._resize());
      }

      // Pause/resume on tab visibility
      this._onVisibility = () => {
        if (document.hidden) {
          this._stopRAF();
        } else if (!this.paused) {
          this.lastFrameTime = 0;
          this._startRAF();
        }
      };
      document.addEventListener("visibilitychange", this._onVisibility);
    }

    _resize() {
      const rect = this.container.getBoundingClientRect();
      const dpr = window.devicePixelRatio || 1;
      const w = Math.floor(rect.width);
      const h = Math.floor(rect.height);
      if (w === 0 || h === 0) return;

      this.canvas.width = w * dpr;
      this.canvas.height = h * dpr;
      this._canvasW = w;
      this._canvasH = h;

      // Cover-fit: dominant axis overflows, other is clipped.
      const charRatio = getMonoCharRatio(this.ctx);
      const fontByCols = w / (this.cols * charRatio);
      const fontByRows = h / this.rows;
      this._fontSize = Math.max(1, Math.ceil(Math.max(fontByCols, fontByRows)));

      this.ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      this.ctx.font = `${this._fontSize}px 'JetBrains Mono', monospace`;
      this.ctx.textBaseline = "top";

      // Measure actual char dimensions
      const metrics = this.ctx.measureText("M");
      this._charW = metrics.width;
      this._charH = this._fontSize;

      // Compute centering offsets (content may overflow on one axis for cover-fit)
      const contentW = this._charW * this.cols;
      const contentH = this._charH * this.rows;
      this._offsetX = (w - contentW) / 2;
      this._offsetY = (h - contentH) / 2;

      this._renderFrame(this.currentFrame);
    }

    _renderFrame(idx) {
      const frame = this.frames[idx];
      if (!frame) return;
      const ctx = this.ctx;
      const cw = this._charW;
      const ch = this._charH;
      const ox = this._offsetX || 0;
      const oy = this._offsetY || 0;

      ctx.clearRect(0, 0, this._canvasW, this._canvasH);

      // Get default text color from CSS
      const defaultColor =
        getComputedStyle(this.container).color || "rgb(200,200,200)";

      const lines = frame.chars.split("\n");
      let colorIdx = 0;

      for (let r = 0; r < lines.length && r < this.rows; r++) {
        const line = lines[r];
        const runes = Array.from(line);
        const y = oy + r * ch;
        let runStart = 0;
        let runColor = "";

        for (let c = 0; c <= runes.length; c++) {
          let cellColor;
          if (c < runes.length) {
            const ci = frame.colors[colorIdx + c] || 0;
            cellColor = ci < this.palette.length ? this.palette[ci] : "";
            if (!cellColor) cellColor = defaultColor;
          }

          if (c === runes.length || (c > runStart && cellColor !== runColor)) {
            // Flush the run
            if (runStart < c) {
              ctx.fillStyle = runColor;
              const text = runes.slice(runStart, c).join("");
              ctx.fillText(text, ox + runStart * cw, y);
            }
            runStart = c;
            runColor = cellColor;
          }
          if (c === 0) runColor = cellColor;
        }

        colorIdx += runes.length;
      }
    }

    _startRAF() {
      if (this.rafId) return;
      this.rafId = requestAnimationFrame((ts) => this._tick(ts));
    }

    _stopRAF() {
      if (this.rafId) {
        cancelAnimationFrame(this.rafId);
        this.rafId = null;
      }
    }

    start() {
      if (matchMedia("(prefers-reduced-motion: reduce)").matches) return;
      this._startRAF();
    }

    stop() {
      this._stopRAF();
      this._ro.disconnect();
      document.removeEventListener("visibilitychange", this._onVisibility);
    }

    _tick(ts) {
      if (!this.container.isConnected) {
        this.stop();
        return;
      }
      this.rafId = requestAnimationFrame((ts) => this._tick(ts));
      if (this.paused) return;
      if (ts - this.lastFrameTime < this.interval) return;
      this.lastFrameTime = ts;
      this.currentFrame = (this.currentFrame + 1) % this.frames.length;
      this._renderFrame(this.currentFrame);
    }

    togglePause() {
      this.paused = !this.paused;
      this.container.classList.toggle("ascii-canvas--paused", this.paused);
      if (this.indicator) this.indicator.hidden = !this.paused;
      if (this.paused) {
        this._stopRAF();
      } else {
        this.lastFrameTime = 0;
        this._startRAF();
      }
    }
  }

  // In-memory cache: framesUrl → parsed ICG wire data.
  const asciiFramesCache = new Map();

  function loadFrames(url) {
    if (asciiFramesCache.has(url))
      return Promise.resolve(asciiFramesCache.get(url));
    return fetch(url)
      .then((r) => r.json())
      .then((data) => {
        asciiFramesCache.set(url, data);
        return data;
      });
  }

  // Per-container setup: lazy frame loading + canvas animation.
  // Called by initAsciiAnimations() and widget picker loadPreview().
  function initAsciiAnimationForCanvas(container) {
    const fps = parseInt(container.dataset.asciiFps, 10) || 12;
    const framesUrl = container.dataset.asciiFramesUrl;

    function startRenderer(data) {
      if (!data.frames || data.frames.length === 0) return;
      const ctrl = new AsciiCanvasRenderer(container, data, fps);
      ctrl.start();
      container.addEventListener("pointerdown", () => ctrl.togglePause());
    }

    if (container.dataset.asciiAutoplay !== undefined) {
      // Dashboard: lazy-load on viewport entry and autoplay.
      const io = new IntersectionObserver(
        (entries, observer) => {
          if (!entries[0].isIntersecting) return;
          observer.disconnect();
          loadFrames(framesUrl)
            .then((data) => startRenderer(data))
            .catch(() => {
              container.classList.remove("ascii-loading");
            });
        },
        { threshold: 0 },
      );
      io.observe(container);
    } else {
      // Gallery: render frame 0 statically, fetch + animate on first click.
      let loaded = false;
      // Eagerly load and render first frame as static preview
      loadFrames(framesUrl).then((data) => {
        if (!data.frames || data.frames.length === 0) {
          container.classList.remove("ascii-loading");
          return;
        }
        const preview = new AsciiCanvasRenderer(container, data, fps);
        preview._renderFrame(0);
        // Don't start animation — just show frame 0
        container.addEventListener("click", () => {
          if (!loaded) {
            loaded = true;
            preview.start();
          } else {
            preview.togglePause();
          }
        });
      }).catch(() => {
        container.classList.remove("ascii-loading");
      });
    }
  }

  function initAsciiAnimations() {
    document
      .querySelectorAll(".ascii-canvas")
      .forEach(initAsciiAnimationForCanvas);
  }

  // ── Tooltips for [data-tip] elements ────────────────────
  const tip = document.createElement("div");
  tip.className = "tooltip";
  document.body.appendChild(tip);
  let tipTarget = null;

  document.addEventListener(
    "pointerenter",
    (e) => {
      if (!e.target || typeof e.target.closest !== "function") return;
      const el = e.target.closest("[data-tip]");
      if (!el || !el.dataset.tip) return;
      // For table cells, only show if content is truncated
      if (el.tagName === "TD") {
        const measure = el.firstElementChild || el;
        if (
          measure.scrollWidth <= measure.clientWidth &&
          el.scrollWidth <= el.clientWidth
        )
          return;
      }
      tipTarget = el;
      tip.textContent = el.dataset.tip;
      const rect = el.getBoundingClientRect();
      tip.style.left = rect.left + "px";
      tip.style.top = rect.top - tip.offsetHeight - 6 + "px";
      tip.classList.add("tooltip--visible");
    },
    true,
  );

  document.addEventListener(
    "pointerleave",
    (e) => {
      if (!e.target || typeof e.target.closest !== "function") return;
      if (e.target.closest("[data-tip]") === tipTarget) {
        tip.classList.remove("tooltip--visible");
        tipTarget = null;
      }
    },
    true,
  );

  // ── Per-character hover effect for .title-text ────────
  document.querySelectorAll(".title-text").forEach((el) => {
    const text = el.textContent;
    el.innerHTML = [...text]
      .map((ch) => `<span class="title-char">${ch}</span>`)
      .join("");

    const h1 = el.closest("h1, .plugin-label") || el.parentElement;
    const chars = [...el.querySelectorAll(".title-char")];
    const basePad = 0.02;
    const peakPad = 0.14;
    const peakStroke = 0.8;
    const baseWeight = 500;
    const dimWeight = 300;

    h1.addEventListener("mousemove", (e) => {
      const h1Rect = h1.getBoundingClientRect();
      chars.forEach((ch) => {
        const rect = ch.getBoundingClientRect();
        const center = rect.left + rect.width / 2;
        const dist = Math.abs(e.clientX - center) / h1Rect.width;
        const t = Math.max(0, 1 - dist * 4);
        ch.style.padding = `0 ${basePad + (peakPad - basePad) * t}em`;
        ch.style.webkitTextStroke = `${peakStroke * t}px var(--text)`;
        ch.style.fontWeight = dimWeight + (baseWeight - dimWeight) * t;
        ch.classList.toggle("title-char--near", t > 0.2);
      });
    });

    h1.addEventListener("mouseleave", () => {
      chars.forEach((ch) => {
        ch.style.padding = `0 ${basePad}em`;
        ch.style.webkitTextStroke = "0px transparent";
        ch.style.fontWeight = baseWeight;
        ch.classList.remove("title-char--near");
      });
    });
  });

  initNav();
  initSortables();
  initWidgetGrid();
  initWidgetPickerConfirm();
  initBreakpointSwitching();
  initBadges();
  initAsciiAnimations();
  initAsciiGallery();

  // ── Widget grid: edit mode, drag-and-drop, controls ────
  function initWidgetGrid() {
    const grid = document.getElementById("widget-grid");
    const editBtn = document.getElementById("edit-toggle");
    const saveBtn = document.getElementById("edit-save");
    const cancelBtn = document.getElementById("edit-cancel");
    const resetBtn = document.getElementById("edit-reset");
    const addCard = document.getElementById("widget-add-card");
    const emptyAddBtn = document.getElementById("empty-add-widget");

    if (emptyAddBtn) {
      emptyAddBtn.addEventListener("click", () => openPicker());
    }

    const gridToggleBtn = document.getElementById("edit-grid-toggle");
    const editActions = document.getElementById("edit-actions");

    if (!grid || !editBtn) return;

    let editing = false;
    let sortable = null;
    let dirty = false;
    let widgetDefs = null;
    let gridOverlay = false;
    let overlayEl = null;

    // Per-breakpoint edit buffers: cols → [{id, size_name}]
    // Captures DOM state when switching away so edits aren't lost
    const editBuffers = {};
    const originalBuffers = {}; // snapshot at edit-enter for reset
    // Track which breakpoints were modified during this edit session
    const modifiedBreakpoints = new Set();
    if (typeof Sortable !== "undefined") {
      sortable = Sortable.create(grid, {
        animation: 200,
        ghostClass: "widget--ghost",
        dragClass: "widget--drag",
        draggable: ".widget:not(.widget-add)",
        handle: ".widget-drag-handle",
        filter: ".widget-remove, .widget-size-btn",
        preventOnFilter: false,
        disabled: true,
        forceFallback: true,
        fallbackOnBody: true,
        swapThreshold: 0.65,
        onEnd: () => {
          dirty = true;
        },
      });
    }

    const isPerBreakpoint = grid.dataset.layoutMode === "per-breakpoint";
    const bpTabs = document.getElementById("breakpoint-tabs");
    const copyBtn = document.getElementById("breakpoint-copy-btn");
    let editCols = parseInt(grid.dataset.activeCols, 10) || 5;

    function enterEditMode() {
      editing = true;
      grid.classList.add("widget-grid--editing");
      editBtn.style.display = "none";
      if (editActions) editActions.style.display = "";
      if (addCard) addCard.style.display = "";
      if (sortable) sortable.option("disabled", false);
      dirty = false;
      populateSizePickers();

      // Clear buffers from any previous edit session
      Object.keys(editBuffers).forEach((k) => delete editBuffers[k]);
      Object.keys(originalBuffers).forEach((k) => delete originalBuffers[k]);
      modifiedBreakpoints.clear();

      // Snapshot current state as original for reset
      const currentCols = parseInt(grid.dataset.activeCols, 10) || 5;
      originalBuffers[currentCols] = readGridState();
      editBuffers[currentCols] = readGridState();

      // Show breakpoint tabs in per-breakpoint mode
      if (isPerBreakpoint && bpTabs) {
        bpTabs.classList.add("visible");
        // Auto-select the breakpoint matching the current viewport
        const vw = window.innerWidth;
        if (vw <= 479) editCols = 2;
        else if (vw <= 767) editCols = 3;
        else editCols = 5;
        bpTabs
          .querySelectorAll(".breakpoint-tab")
          .forEach((t) =>
            t.classList.toggle(
              "active",
              parseInt(t.dataset.cols, 10) === editCols,
            ),
          );
        applyGridConstraint(editCols);

        // Fetch the correct breakpoint's widgets if not already showing
        if (currentCols !== editCols) {
          loadBreakpointGrid(editCols);
        }
      }
    }

    function exitEditMode() {
      editing = false;
      grid.classList.remove("widget-grid--editing");
      grid.classList.remove("widget-grid--constrained");
      grid.classList.remove("widget-grid--show-grid");
      removeGridOverlay();
      grid.removeAttribute("data-sim-cols");
      grid.style.gridTemplateColumns = "";
      gridOverlay = false;
      editBtn.style.display = "";
      if (editActions) editActions.style.display = "none";
      if (gridToggleBtn) gridToggleBtn.classList.remove("active");
      if (addCard) addCard.style.display = "none";
      if (sortable) sortable.option("disabled", true);
      if (bpTabs) bpTabs.classList.remove("visible");
    }

    // Read current widget state from the DOM
    function readGridState() {
      return [...grid.querySelectorAll(".widget[data-widget-id]")].map(
        (el) => ({ id: el.dataset.widgetId, size_name: el.dataset.size }),
      );
    }

    // Capture current DOM into the edit buffer for the given cols
    function captureToBuffer(cols) {
      editBuffers[cols] = readGridState();
    }

    // Render widgets from a buffer into the grid (used when switching tabs from buffer)
    function applyBufferToGrid(widgets, cols) {
      // Remove existing widgets
      grid
        .querySelectorAll(".widget:not(.widget-add)")
        .forEach((w) => w.remove());

      if (!widgets || widgets.length === 0) {
        applyGridConstraint(cols);
        return Promise.resolve();
      }

      // Fetch rendered HTML for these widgets at this breakpoint
      const filter = getActiveFilter();
      return fetch(`/?cols=${cols}&filter=${encodeURIComponent(filter)}`)
        .then((r) => r.text())
        .then((html) => {
          const doc = new DOMParser().parseFromString(html, "text/html");
          const newGrid = doc.getElementById("widget-grid");
          if (!newGrid) return;

          // Build a lookup of rendered widgets by ID
          const rendered = {};
          newGrid.querySelectorAll(".widget:not(.widget-add)").forEach((w) => {
            rendered[w.dataset.widgetId] = w;
          });

          const ac = document.getElementById("widget-add-card");
          // Insert in buffer order, matching by ID
          widgets.forEach((bw) => {
            const el = rendered[bw.id];
            if (el) {
              // Apply the buffered size (may differ from server)
              el.dataset.size = bw.size_name;
              if (ac) grid.insertBefore(el, ac);
              else grid.appendChild(el);
            }
          });

          if (sortable) sortable.option("disabled", false);
          populateSizePickers();
          applyGridConstraint(cols);
        })
        .catch((err) => console.error("Failed to apply buffer:", err));
    }

    function applyGridConstraint(cols) {
      grid.style.gridTemplateColumns = `repeat(${cols}, 1fr)`;
      if (cols < 5) {
        grid.classList.add("widget-grid--constrained");
        grid.dataset.simCols = cols;
      } else {
        grid.classList.remove("widget-grid--constrained");
        grid.removeAttribute("data-sim-cols");
      }
      // Rebuild overlay if it's showing so it matches new cols
      if (gridOverlay) buildGridOverlay();
    }

    // Build / tear down the grid overlay with real dashed-border cells
    function buildGridOverlay() {
      removeGridOverlay();
      overlayEl = document.createElement("div");
      overlayEl.className = "grid-overlay";
      // Match the grid's current column setting
      const cols = parseInt(
        grid.style.gridTemplateColumns?.match(/repeat\((\d+)/)?.[1] || "5",
        10,
      );
      overlayEl.style.gridTemplateColumns = `repeat(${cols}, 1fr)`;
      // Fill enough rows to cover the grid — count widget rows + a few extra
      const rows = Math.max(4, Math.ceil(grid.scrollHeight / 130));
      const cellCount = cols * rows;
      for (let i = 0; i < cellCount; i++) {
        const cell = document.createElement("div");
        cell.className = "grid-overlay-cell";
        overlayEl.appendChild(cell);
      }
      grid.prepend(overlayEl);
    }

    function removeGridOverlay() {
      if (overlayEl) {
        overlayEl.remove();
        overlayEl = null;
      }
    }

    // Grid overlay toggle
    if (gridToggleBtn) {
      gridToggleBtn.addEventListener("click", () => {
        gridOverlay = !gridOverlay;
        if (gridOverlay) {
          grid.classList.add("widget-grid--show-grid");
          buildGridOverlay();
        } else {
          grid.classList.remove("widget-grid--show-grid");
          removeGridOverlay();
        }
        gridToggleBtn.classList.toggle("active", gridOverlay);
      });
    }

    // Fetch and replace grid contents for a given breakpoint
    function loadBreakpointGrid(cols) {
      const filter = getActiveFilter();
      return fetch(`/?cols=${cols}&filter=${encodeURIComponent(filter)}`)
        .then((r) => r.text())
        .then((html) => {
          const doc = new DOMParser().parseFromString(html, "text/html");
          const newGrid = doc.getElementById("widget-grid");
          if (!newGrid) return;
          // Replace grid inner content (widgets), keep add card
          const currentWidgets = grid.querySelectorAll(
            ".widget:not(.widget-add)",
          );
          currentWidgets.forEach((w) => w.remove());
          const ac = document.getElementById("widget-add-card");
          newGrid.querySelectorAll(".widget:not(.widget-add)").forEach((w) => {
            if (ac) grid.insertBefore(w, ac);
            else grid.appendChild(w);
          });
          // Re-init sortable items
          if (sortable) sortable.option("disabled", false);
          populateSizePickers();
          applyGridConstraint(cols);
        })
        .catch((err) =>
          console.error("Failed to load breakpoint layout:", err),
        );
    }

    // Breakpoint tab click — capture current, switch to target
    if (bpTabs) {
      bpTabs.addEventListener("click", (e) => {
        const tab = e.target.closest(".breakpoint-tab");
        if (!tab || !editing) return;
        const cols = parseInt(tab.dataset.cols, 10);
        if (!cols || cols === editCols) return;

        // Capture current breakpoint's DOM state before switching
        captureToBuffer(editCols);
        modifiedBreakpoints.add(String(editCols));

        const prevCols = editCols;
        editCols = cols;
        bpTabs
          .querySelectorAll(".breakpoint-tab")
          .forEach((t) => t.classList.toggle("active", t === tab));

        // Load from buffer if we have edits, otherwise fetch from server
        if (editBuffers[cols]) {
          applyBufferToGrid(editBuffers[cols], cols);
        } else {
          loadBreakpointGrid(cols).then(() => {
            // Snapshot the freshly loaded state as original for reset
            if (!originalBuffers[cols]) {
              originalBuffers[cols] = readGridState();
            }
            editBuffers[cols] = readGridState();
          });
        }
      });
    }

    // Copy-from popover menu
    const copyMenu = document.getElementById("breakpoint-copy-menu");
    if (copyBtn && copyMenu) {
      const BP_LABELS = { 5: "Desktop (5)", 3: "Tablet (3)", 2: "Mobile (2)" };

      copyBtn.addEventListener("click", (e) => {
        e.stopPropagation();
        if (!editing || !isPerBreakpoint) return;

        const otherCols = [5, 3, 2].filter((c) => c !== editCols);
        copyMenu.innerHTML = otherCols
          .map(
            (c) =>
              `<button class="breakpoint-copy-option" data-source-cols="${c}">${BP_LABELS[c]}</button>`,
          )
          .join("");
        copyMenu.classList.toggle("open");
      });

      copyMenu.addEventListener("click", (e) => {
        const opt = e.target.closest(".breakpoint-copy-option");
        if (!opt) return;
        const sourceCols = parseInt(opt.dataset.sourceCols, 10);
        copyMenu.classList.remove("open");

        // Copy from source buffer (or fetch if not yet loaded)
        const doApply = (widgets) => {
          editBuffers[editCols] = [...widgets];
          modifiedBreakpoints.add(String(editCols));
          applyBufferToGrid(editBuffers[editCols], editCols);
        };

        if (editBuffers[sourceCols]) {
          doApply(editBuffers[sourceCols]);
        } else {
          // Fetch source breakpoint, then copy
          const filter = getActiveFilter();
          fetch(`/?cols=${sourceCols}&filter=${encodeURIComponent(filter)}`)
            .then((r) => r.text())
            .then((html) => {
              const doc = new DOMParser().parseFromString(html, "text/html");
              const srcGrid = doc.getElementById("widget-grid");
              if (!srcGrid) return;
              const widgets = [
                ...srcGrid.querySelectorAll(".widget[data-widget-id]"),
              ].map((el) => ({
                id: el.dataset.widgetId,
                size_name: el.dataset.size,
              }));
              doApply(widgets);
            })
            .catch((err) => console.error("Failed to copy layout:", err));
        }
      });

      // Close menu when clicking elsewhere
      document.addEventListener("click", () => {
        copyMenu.classList.remove("open");
      });
    }

    editBtn.addEventListener("click", enterEditMode);

    // Save — collect all modified breakpoints and bulk-save
    if (saveBtn) {
      saveBtn.addEventListener("click", () => {
        // Capture the currently visible breakpoint
        captureToBuffer(editCols);
        modifiedBreakpoints.add(String(editCols));

        if (isPerBreakpoint) {
          // Build layouts map from all modified buffers
          const layouts = {};
          modifiedBreakpoints.forEach((cols) => {
            if (editBuffers[cols]) {
              layouts[cols] = editBuffers[cols];
            }
          });
          fetch("/api/dashboard/widgets", {
            method: "PUT",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ layouts }),
          })
            .then(() => window.location.reload())
            .catch((err) => console.error("Failed to save dashboard:", err));
        } else {
          const widgets = readGridState();
          fetch("/api/dashboard/widgets", {
            method: "PUT",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ widgets }),
          })
            .then(() => window.location.reload())
            .catch((err) => console.error("Failed to save dashboard:", err));
        }
      });
    }

    // Cancel — discard all changes by reloading
    if (cancelBtn) {
      cancelBtn.addEventListener("click", () => {
        window.location.reload();
      });
    }

    // Reset — restore only the current breakpoint to its original state
    if (resetBtn) {
      resetBtn.addEventListener("click", () => {
        const key = editCols;
        if (originalBuffers[key]) {
          editBuffers[key] = [...originalBuffers[key]];
          modifiedBreakpoints.delete(String(key));
          applyBufferToGrid(editBuffers[key], key);
        } else {
          // No original snapshot — reload from server
          delete editBuffers[key];
          modifiedBreakpoints.delete(String(key));
          loadBreakpointGrid(key);
        }
      });
    }

    // Remove widget — controls are inside the widget, grid handles clicks
    grid.addEventListener("click", (e) => {
      const removeBtn = e.target.closest(".widget-remove");
      if (!removeBtn) return;
      const widgetEl = removeBtn.closest(".widget");
      if (widgetEl) {
        widgetEl.remove();
        dirty = true;
      }
    });

    // Size picker — controls are inside the widget, grid handles clicks
    grid.addEventListener("click", (e) => {
      const sizeBtn = e.target.closest(".widget-size-btn");
      if (!sizeBtn) return;
      const widgetEl = sizeBtn.closest(".widget");
      const id = widgetEl.dataset.widgetId;
      const sizeName = sizeBtn.dataset.sizeName;

      // Resolve instance IDs (e.g. "spacer:123") to base ID
      const baseId = id.includes(":") ? id.split(":")[0] : id;
      const def =
        widgetDefs && widgetDefs.find((w) => w.id === baseId || w.id === id);
      const sizeOpt = def && def.sizes.find((s) => s.name === sizeName);
      if (!sizeOpt) return;

      widgetEl.dataset.size = sizeName;
      widgetEl.style.gridColumn = `span ${sizeOpt.w}`;
      widgetEl.style.gridRow = `span ${sizeOpt.h}`;

      widgetEl
        .querySelectorAll(".widget-size-btn")
        .forEach((b) =>
          b.classList.toggle("active", b.dataset.sizeName === sizeName),
        );

      // Spacers render nothing — skip the preview fetch
      if (!widgetEl.classList.contains("widget-spacer")) {
        const filter = getActiveFilter();
        fetch(
          `/api/widgets/${encodeURIComponent(baseId)}/preview?size=${encodeURIComponent(sizeName)}&filter=${encodeURIComponent(filter)}`,
        )
          .then((r) => r.json())
          .then((data) => {
            const fill = widgetEl.querySelector(".widget-fill");
            if (fill) {
              const temp = document.createElement("div");
              temp.innerHTML = data.html;
              const newFill = temp.querySelector(".widget-fill");
              if (newFill) fill.replaceWith(newFill);
            }
          })
          .catch((err) =>
            console.error("Failed to fetch widget preview:", err),
          );
      }

      dirty = true;
    });

    // Add widget card click
    if (addCard) {
      addCard.addEventListener("click", () => openPicker());
    }

    function populateSizePickers() {
      fetch("/api/widgets")
        .then((r) => r.json())
        .then((widgets) => {
          widgetDefs = widgets;
          const widgetMap = {};
          widgets.forEach((w) => {
            widgetMap[w.id] = w;
          });

          grid
            .querySelectorAll(".widget-size-picker[data-widget-id]")
            .forEach((picker) => {
              const id = picker.dataset.widgetId;
              // Resolve instance IDs (e.g. "spacer:123") to base ID
              const baseId = id.includes(":") ? id.split(":")[0] : id;
              const def = widgetMap[baseId] || widgetMap[id];
              if (!def) return;
              const currentSize = picker.closest(".widget").dataset.size;
              picker.innerHTML = def.sizes
                .map(
                  (s) =>
                    `<button class="widget-size-btn${s.name === currentSize ? " active" : ""}" data-size-name="${s.name}">${s.name}</button>`,
                )
                .join("");
            });
        })
        .catch((err) =>
          console.error("Failed to load widget definitions:", err),
        );
    }
  }

  // ── Widget picker modal ─────────────────────────────────
  // Flow: Plugin list → Widget list → Preview with size picker
  function openPicker() {
    const overlay = document.getElementById("widget-picker-overlay");
    const listEl = document.getElementById("widget-picker-list");
    const previewEl = document.getElementById("widget-picker-preview");
    const previewArea = document.getElementById("widget-picker-preview-area");
    const previewTitle = document.getElementById("widget-picker-preview-title");
    const sizesEl = document.getElementById("widget-picker-sizes");
    const backBtn = document.getElementById("widget-picker-back");
    const pickerTitle = document.querySelector(".widget-picker-title");

    if (!overlay) return;

    widgetPickerSelection.widget = null;
    widgetPickerSelection.size = null;

    let allWidgets = [];
    let selectedPlugin = null;
    let selectedWidget = null;
    let selectedSize = null;
    const multiInstance = new Set(["spacer", "ascii"]);

    overlay.classList.add("open");
    backBtn.style.display = "none";
    showPluginList();

    // Get current filter
    const filter = getActiveFilter();

    // Fetch available widgets and show plugin groups
    fetch("/api/widgets")
      .then((r) => r.json())
      .then((widgets) => {
        allWidgets = widgets;
        renderPluginList();
      })
      .catch((err) => console.error("Failed to load widgets:", err));

    function showPluginList() {
      listEl.style.display = "";
      previewEl.style.display = "none";
      backBtn.style.display = "none";
      if (pickerTitle) pickerTitle.textContent = "Add Widget";
      selectedPlugin = null;
      selectedWidget = null;
      selectedSize = null;
    }

    function renderPluginList() {
      const pinned = new Set(
        [...document.querySelectorAll(".widget[data-widget-id]")].map(
          (el) => el.dataset.widgetId,
        ),
      );

      // Group by plugin_id
      const groups = {};
      allWidgets.forEach((w) => {
        if (!multiInstance.has(w.plugin_id) && pinned.has(w.id)) return;
        if (!groups[w.plugin_id]) groups[w.plugin_id] = [];
        groups[w.plugin_id].push(w);
      });

      const pluginIds = Object.keys(groups);
      if (pluginIds.length === 0) {
        listEl.innerHTML =
          '<p style="color:var(--text-3);padding:1rem">All widgets are already pinned.</p>';
        return;
      }

      listEl.innerHTML = pluginIds
        .map((pid) => {
          const count = groups[pid].length;
          const meta = PLUGIN_META[pid];
          return `
          <div class="widget-picker-item" data-plugin-id="${pid}">
            ${meta?.icon || ""}
            <div>
              <div class="widget-picker-item-name">${meta?.name || pid}</div>
              <div class="widget-picker-item-desc">${count} widget${count !== 1 ? "s" : ""} available</div>
            </div>
          </div>`;
        })
        .join("");

      listEl
        .querySelectorAll(".widget-picker-item[data-plugin-id]")
        .forEach((item) => {
          item.addEventListener("click", () => {
            selectedPlugin = item.dataset.pluginId;
            renderWidgetList(groups[selectedPlugin]);
          });
        });
    }

    function renderWidgetList(widgets) {
      backBtn.style.display = "";
      if (pickerTitle)
        pickerTitle.textContent =
          PLUGIN_META[selectedPlugin]?.name || selectedPlugin;
      listEl.innerHTML = widgets
        .map(
          (w) => `
        <div class="widget-picker-item" data-widget-id="${w.id}">
          <div>
            <div class="widget-picker-item-name">${w.name}</div>
            <div class="widget-picker-item-desc">${w.description}</div>
          </div>
          <div class="widget-picker-item-sizes">
            ${w.sizes.map((s) => `<span class="widget-picker-size-dot" style="width:${s.w * 12}px;height:${s.h * 12}px"></span>`).join("")}
          </div>
        </div>
      `,
        )
        .join("");

      listEl
        .querySelectorAll(".widget-picker-item[data-widget-id]")
        .forEach((item) => {
          item.addEventListener("click", () => {
            const id = item.dataset.widgetId;
            selectedWidget = allWidgets.find((w) => w.id === id);
            if (!selectedWidget) return;
            showPreview();
          });
        });
    }

    function showPreview() {
      listEl.style.display = "none";
      previewEl.style.display = "";
      previewTitle.textContent = selectedWidget.name;

      selectedSize = selectedWidget.sizes[0].name;
      widgetPickerSelection.widget = selectedWidget;
      widgetPickerSelection.size = selectedSize;
      widgetPickerSelection.filter = filter;

      sizesEl.innerHTML = selectedWidget.sizes
        .map(
          (s) =>
            `<button class="widget-size-btn${s.name === selectedSize ? " active" : ""}" data-size-name="${s.name}">${s.name} (${s.w}\u00d7${s.h})</button>`,
        )
        .join("");

      loadPreview(selectedWidget.id, selectedSize, filter);

      sizesEl.querySelectorAll(".widget-size-btn").forEach((btn) => {
        btn.addEventListener("click", () => {
          selectedSize = btn.dataset.sizeName;
          widgetPickerSelection.size = selectedSize;
          sizesEl
            .querySelectorAll(".widget-size-btn")
            .forEach((b) => b.classList.toggle("active", b === btn));
          loadPreview(selectedWidget.id, selectedSize, filter);
        });
      });
    }

    function loadPreview(id, size, f) {
      previewArea.innerHTML = '<p style="color:var(--text-3)">Loading...</p>';
      fetch(
        `/api/widgets/${encodeURIComponent(id)}/preview?size=${encodeURIComponent(size)}&filter=${encodeURIComponent(f)}`,
      )
        .then((r) => r.json())
        .then((data) => {
          // Compute the exact pixel dimensions this widget occupies on the dashboard grid.
          const GRID_COL_W = 161,
            GRID_ROW_H = 130,
            GRID_GAP = 12;
          const W = data.w || 1,
            H = data.h || 1;
          const pxW = W * GRID_COL_W + (W - 1) * GRID_GAP;
          const pxH = H * GRID_ROW_H + (H - 1) * GRID_GAP;
          // Wrap in a .widget at true dashboard size so layout + font scaling matches exactly.
          const wrapper = document.createElement("div");
          wrapper.className = "widget";
          wrapper.style.width = pxW + "px";
          wrapper.style.height = pxH + "px";
          wrapper.innerHTML = data.html;
          previewArea.innerHTML = "";
          previewArea.appendChild(wrapper);
          previewArea
            .querySelectorAll(".ascii-canvas")
            .forEach(initAsciiAnimationForCanvas);
        })
        .catch(() => {
          previewArea.innerHTML =
            '<p style="color:var(--remove)">Failed to load preview.</p>';
        });
    }

    // Back — context-aware: preview → widget list → plugin list
    backBtn.addEventListener("click", () => {
      if (previewEl.style.display !== "none") {
        // From preview → back to widget list
        listEl.style.display = "";
        previewEl.style.display = "none";
        const pinned = new Set(
          [...document.querySelectorAll(".widget[data-widget-id]")].map(
            (el) => el.dataset.widgetId,
          ),
        );
        const pluginWidgets = allWidgets.filter(
          (w) =>
            w.plugin_id === selectedPlugin &&
            (multiInstance.has(w.plugin_id) || !pinned.has(w.id)),
        );
        renderWidgetList(pluginWidgets);
        selectedWidget = null;
        selectedSize = null;
      } else if (selectedPlugin) {
        // From widget list → back to plugin list
        renderPluginList();
      }
    });

    // (Confirm pin is handled by a single listener registered in DOMContentLoaded)
  }

  function closeWidgetPicker() {
    const overlay = document.getElementById("widget-picker-overlay");
    if (overlay) overlay.classList.remove("open");
  }

  function initWidgetPickerConfirm() {
    const overlay = document.getElementById("widget-picker-overlay");
    const closeBtn = document.getElementById("widget-picker-close");
    const confirmBtn = document.getElementById("widget-picker-confirm");
    if (!overlay) return;

    if (closeBtn) closeBtn.addEventListener("click", closeWidgetPicker);
    overlay.addEventListener("click", (e) => {
      if (e.target === overlay) closeWidgetPicker();
    });

    if (!confirmBtn) return;
    confirmBtn.addEventListener("click", () => {
      const selectedWidget = widgetPickerSelection.widget;
      const selectedSize = widgetPickerSelection.size;
      const filter = widgetPickerSelection.filter || getActiveFilter();
      if (!selectedWidget || !selectedSize) return;
      const sizeOpt = selectedWidget.sizes.find((s) => s.name === selectedSize);
      if (!sizeOpt) return;

      // Spacers and ASCII widgets get unique instance IDs so multiple can coexist
      const needsInstance = ["spacer", "ascii"].includes(
        selectedWidget.plugin_id,
      );
      const widgetId = needsInstance
        ? selectedWidget.id + ":" + Date.now()
        : selectedWidget.id;

      const grid = document.getElementById("widget-grid");
      if (!grid) {
        fetch("/api/dashboard/widgets", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ id: widgetId, size_name: selectedSize }),
        })
          .then(() => window.location.reload())
          .catch((err) => console.error("Failed to pin widget:", err));
        return;
      }

      const addWidget = (html) => {
        const grid = document.getElementById("widget-grid");
        const addCard = document.getElementById("widget-add-card");
        if (!grid) return;

        const pid = selectedWidget.plugin_id;
        const meta = PLUGIN_META[pid];

        const div = document.createElement("div");
        const isSpacer = selectedWidget.plugin_id === "spacer";
        div.className = "widget" + (isSpacer ? " widget-spacer" : "");
        div.dataset.widgetId = widgetId;
        div.dataset.size = selectedSize;
        div.style.gridColumn = `span ${sizeOpt.w}`;
        div.style.gridRow = `span ${sizeOpt.h}`;
        div.innerHTML = `
          <div class="widget-controls">
            <span class="widget-drag-handle" title="Drag to reorder" aria-hidden="true">⋮⋮</span>
            <button class="widget-remove" data-widget-id="${widgetId}" title="Remove widget">&times;</button>
          </div>
          <div class="widget-size-picker" data-widget-id="${widgetId}"></div>
          ${!isSpacer && meta?.badgeIcon ? `<div class="widget-badge" title="${meta.name || pid}">${meta.badgeIcon}</div>` : ""}
          ${html}`;

        if (addCard) grid.insertBefore(div, addCard);
        else grid.appendChild(div);

        const picker = div.querySelector(".widget-size-picker");
        if (picker) {
          picker.innerHTML = selectedWidget.sizes
            .map(
              (s) =>
                `<button class="widget-size-btn${s.name === selectedSize ? " active" : ""}" data-size-name="${s.name}">${s.name}</button>`,
            )
            .join("");
        }

        closeWidgetPicker();
      };

      if (selectedWidget.plugin_id === "spacer") {
        addWidget("");
      } else {
        fetch(
          `/api/widgets/${encodeURIComponent(selectedWidget.id)}/preview?size=${encodeURIComponent(selectedSize)}&filter=${encodeURIComponent(filter)}`,
        )
          .then((r) => r.json())
          .then((data) => addWidget(data.html))
          .catch((err) =>
            console.error("Failed to fetch widget preview:", err),
          );
      }
    });
  }

  // ── Breakpoint switching (per-breakpoint mode, runtime) ──
  function initBreakpointSwitching() {
    const grid = document.getElementById("widget-grid");
    if (!grid || grid.dataset.layoutMode !== "per-breakpoint") return;

    const BREAKPOINTS = [
      { cols: 5, query: "(min-width: 768px)" },
      { cols: 3, query: "(min-width: 480px) and (max-width: 767px)" },
      { cols: 2, query: "(max-width: 479px)" },
    ];

    // Cache fetched HTML per breakpoint
    const cache = {};
    let currentCols = parseInt(grid.dataset.activeCols, 10) || 5;

    function switchBreakpoint(cols) {
      if (cols === currentCols) return;
      // Don't switch while editing
      if (grid.classList.contains("widget-grid--editing")) return;

      currentCols = cols;
      const filter = getActiveFilter();
      const cacheKey = `${cols}:${filter}`;

      if (cache[cacheKey]) {
        applyBreakpointHTML(cache[cacheKey], cols);
        return;
      }

      fetch(`/?cols=${cols}&filter=${encodeURIComponent(filter)}`)
        .then((r) => r.text())
        .then((html) => {
          cache[cacheKey] = html;
          // Check we're still on this breakpoint
          if (currentCols === cols) {
            applyBreakpointHTML(html, cols);
          }
        })
        .catch((err) => console.error("Failed to load breakpoint:", err));
    }

    function applyBreakpointHTML(html, cols) {
      const doc = new DOMParser().parseFromString(html, "text/html");
      const newGrid = doc.getElementById("widget-grid");
      if (!newGrid) return;

      // Replace grid contents
      grid.innerHTML = newGrid.innerHTML;
      grid.dataset.activeCols = cols;

      // Re-initialize ASCII animations for the new DOM nodes.
      // Old controllers self-stop via the !canvas.isConnected check in _tick.
      initAsciiAnimations();
    }

    BREAKPOINTS.forEach((bp) => {
      const mql = window.matchMedia(bp.query);
      const handler = (e) => {
        if (e.matches) switchBreakpoint(bp.cols);
      };
      mql.addEventListener("change", handler);
      // Check initial state
      if (mql.matches && bp.cols !== currentCols) {
        switchBreakpoint(bp.cols);
      }
    });
  }

  // ── ASCII Gallery page ──────────────────────────────────
  function initAsciiGallery() {
    if (!document.querySelector(".ascii-gallery-grid, .ascii-empty")) return;

    // Font scaling is handled by CSS container queries on .ascii-card-thumb —
    // no ResizeObserver needed for gallery thumbnails.

    // Search and size filter submit the form for server-side pagination.
    const filterForm = document.querySelector(".ascii-filter-form");
    const searchInput = document.querySelector(".ascii-search");
    const sizeFilter = document.querySelector(".ascii-size-filter");

    if (filterForm) {
      // Debounced search: submit form after user stops typing.
      let searchDebounce = null;
      if (searchInput) {
        searchInput.addEventListener("input", () => {
          clearTimeout(searchDebounce);
          searchDebounce = setTimeout(() => filterForm.submit(), 400);
        });
      }
      // Size filter: submit immediately on change.
      if (sizeFilter) {
        sizeFilter.addEventListener("change", () => filterForm.submit());
      }
    }

    // Gallery variant delete buttons
    document.querySelectorAll(".ascii-delete-variant-btn").forEach((btn) => {
      btn.addEventListener("click", () => {
        const name = btn.dataset.name;
        const size = btn.dataset.size;
        if (!confirm(`Delete variant "${size}" of "${name}"?`)) return;
        fetch(
          `/api/ascii/animations/${encodeURIComponent(name)}/${encodeURIComponent(size)}`,
          { method: "DELETE" },
        )
          .then((r) => {
            if (r.status === 204 || r.ok) {
              const item = document.querySelector(
                `.ascii-gallery-item[data-name="${CSS.escape(name)}"][data-size="${CSS.escape(size)}"]`,
              );
              if (item) item.remove();
              showToast(`Deleted "${name}" (${size})`, { type: "success" });
            } else {
              return r.json().then((data) => {
                throw new Error(data.error || "Delete failed");
              });
            }
          })
          .catch((err) => showToast(err.message, { type: "error" }));
      });
    });

    // ── Gallery Import ────────────────────────────────────
    const importBtn = document.getElementById("ascii-import-btn");
    const importDropdown = document.getElementById("ascii-import-dropdown");
    const importFileInput = document.getElementById("ascii-import-file-input");
    const importResult = document.getElementById("ascii-import-result");
    const importResultText = document.getElementById(
      "ascii-import-result-text",
    );
    const importResultClose = document.getElementById(
      "ascii-import-result-close",
    );

    if (importBtn && importDropdown) {
      importBtn.addEventListener("click", (e) => {
        e.stopPropagation();
        importDropdown.hidden = !importDropdown.hidden;
      });
      document.addEventListener("click", () => {
        importDropdown.hidden = true;
      });
      importDropdown
        .querySelectorAll(".ascii-import-option:not([disabled])")
        .forEach((opt) => {
          opt.addEventListener("click", () => {
            importDropdown.hidden = true;
            importFileInput?.click();
          });
        });
    }

    if (importFileInput) {
      importFileInput.addEventListener("change", async () => {
        const files = importFileInput.files;
        if (!files || files.length === 0) return;

        const form = new FormData();
        for (const file of files) {
          // Use the relative path as the field name (not filename). Go's multipart
          // parser sanitizes filenames by stripping directories; field names are preserved.
          form.append(file.webkitRelativePath || file.name, file);
        }

        try {
          const res = await fetch("/api/ascii/import", {
            method: "POST",
            body: form,
          });
          const data = await res.json();

          if (res.status === 409 && data.conflicts?.length > 0) {
            const names = data.conflicts.join(", ");
            if (
              confirm(
                `These animations already exist: ${names}.\nOverwrite them?`,
              )
            ) {
              const res2 = await fetch("/api/ascii/import?overwrite=true", {
                method: "POST",
                body: form,
              });
              const data2 = await res2.json();
              showImportResult(data2);
              if (data2.imported?.length > 0) location.reload();
            }
          } else if (res.ok) {
            showImportResult(data);
            if (data.imported?.length > 0) location.reload();
          } else {
            showImportResult({ error: data.error || "Import failed" });
          }
        } catch (err) {
          showImportResult({ error: "Import failed: " + err.message });
        }
        importFileInput.value = "";
      });
    }

    function showImportResult(data) {
      if (!importResult || !importResultText) return;
      let msg = "";
      if (data.error) {
        msg = data.error;
        importResult.classList.add("error");
      } else {
        const parts = [];
        if (data.imported?.length)
          parts.push(
            `Imported: ${data.imported.map((a) => a.name).join(", ")}`,
          );
        if (data.skipped?.length)
          parts.push(
            `Skipped: ${data.skipped.map((a) => `${a.name} (${a.reason})`).join(", ")}`,
          );
        msg = parts.join(" · ") || "Nothing imported";
        importResult.classList.remove("error");
      }
      importResultText.textContent = msg;
      importResult.hidden = false;
    }

    if (importResultClose)
      importResultClose.addEventListener("click", () => {
        importResult.hidden = true;
      });

    // ── Gallery Export ────────────────────────────────────
    const galleryExportBtn = document.getElementById("ascii-export-btn");
    const exportOverlay = document.getElementById("ascii-export-overlay");
    const exportClose = document.getElementById("ascii-export-close");
    const exportCancel = document.getElementById("ascii-export-cancel");
    const exportDownload = document.getElementById("ascii-export-download");
    const exportList = document.getElementById("ascii-export-list");
    const exportSelectAll = document.getElementById("ascii-export-select-all");
    const exportPackMeta = document.getElementById("ascii-export-pack-meta");
    const exportPackName = document.getElementById("ascii-export-pack-name");
    const exportPackVersion = document.getElementById(
      "ascii-export-pack-version",
    );
    const exportPackAuthor = document.getElementById(
      "ascii-export-pack-author",
    );
    const exportPackDescription = document.getElementById(
      "ascii-export-pack-description",
    );
    const exportPackLicense = document.getElementById(
      "ascii-export-pack-license",
    );
    const exportErrors = document.getElementById("ascii-export-errors");

    let localAnimations = [];

    async function openExportModal() {
      try {
        const res = await fetch("/api/ascii/animations");
        const all = await res.json();
        localAnimations = all.filter((a) => !a.source);
      } catch (err) {
        showToast("Failed to load animations: " + err.message, {
          type: "error",
        });
        return;
      }

      if (exportList) {
        exportList.innerHTML = "";
        localAnimations.forEach((anim) => {
          const sizes = anim.variants?.map((v) => v.size).join(" · ") || "";
          const label = document.createElement("label");
          label.className = "ascii-export-item";
          label.innerHTML = `<input type="checkbox" class="app-checkbox" value="${escapeHtml(anim.name)}" checked> <span class="ascii-export-item-name">${escapeHtml(anim.name)}</span> <span class="ascii-export-item-sizes">${escapeHtml(sizes)}</span>`;
          exportList.appendChild(label);
        });
      }

      updateExportPackMeta();
      if (exportOverlay) exportOverlay.classList.add("open");
    }

    function updateExportPackMeta() {
      if (!exportList || !exportPackMeta || !exportSelectAll) return;
      const checked = exportList.querySelectorAll(
        "input[type=checkbox]:checked",
      );
      exportPackMeta.style.display = checked.length >= 2 ? "" : "none";
      exportSelectAll.checked = checked.length === localAnimations.length;
      exportSelectAll.indeterminate =
        checked.length > 0 && checked.length < localAnimations.length;
    }

    if (galleryExportBtn)
      galleryExportBtn.addEventListener("click", openExportModal);

    if (exportClose)
      exportClose.addEventListener("click", () =>
        exportOverlay?.classList.remove("open"),
      );
    if (exportCancel)
      exportCancel.addEventListener("click", () =>
        exportOverlay?.classList.remove("open"),
      );

    if (exportOverlay) {
      exportOverlay.addEventListener("click", (e) => {
        if (e.target === exportOverlay) exportOverlay.classList.remove("open");
      });
    }

    if (exportSelectAll) {
      exportSelectAll.addEventListener("change", () => {
        exportList?.querySelectorAll("input[type=checkbox]").forEach((cb) => {
          cb.checked = exportSelectAll.checked;
        });
        updateExportPackMeta();
      });
    }

    if (exportList) {
      exportList.addEventListener("change", updateExportPackMeta);
    }

    if (exportDownload) {
      exportDownload.addEventListener("click", async () => {
        if (!exportList || !exportErrors) return;
        const checked = [
          ...exportList.querySelectorAll("input[type=checkbox]:checked"),
        ].map((cb) => cb.value);
        if (checked.length === 0) {
          exportErrors.textContent = "Select at least one animation.";
          return;
        }
        exportErrors.textContent = "";

        const body = {
          author: exportPackAuthor?.value || "",
          name: exportPackName?.value || "export",
          description: exportPackDescription?.value || "",
          version: exportPackVersion?.value || "1.0.0",
          license: exportPackLicense?.value || "",
          animations: checked,
        };

        try {
          const res = await fetch("/api/ascii/export", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(body),
          });

          if (!res.ok) {
            const data = await res.json().catch(() => ({}));
            exportErrors.textContent = data.error || "Export failed.";
            return;
          }

          const blob = await res.blob();
          const url = URL.createObjectURL(blob);
          const a = document.createElement("a");
          a.href = url;
          a.download =
            (checked.length === 1 ? checked[0] : body.name || "export") +
            ".zip";
          document.body.appendChild(a);
          a.click();
          a.remove();
          URL.revokeObjectURL(url);
          exportOverlay?.classList.remove("open");
        } catch (err) {
          exportErrors.textContent = "Export failed: " + err.message;
        }
      });
    }

    initAsciiModal();
  }

  // ── ASCII CRUD Modal ────────────────────────────────────
  function initAsciiModal() {
    const overlay = document.getElementById("ascii-modal-overlay");
    if (!overlay) return;

    const modal = overlay.querySelector(".ascii-modal");
    const closeBtn = document.getElementById("ascii-modal-close");
    const cancelBtn = document.getElementById("ascii-modal-cancel");
    const saveBtn = document.getElementById("ascii-modal-save");
    const titleEl = document.getElementById("ascii-modal-title");
    const errorsEl = document.getElementById("ascii-modal-errors");

    const nameInput = document.getElementById("ascii-form-name");
    const sizeInput = document.getElementById("ascii-form-size");
    const colsInput = document.getElementById("ascii-form-cols");
    const rowsInput = document.getElementById("ascii-form-rows");
    const fpsInput = document.getElementById("ascii-form-fps");
    const paletteRowsEl = document.getElementById("ascii-palette-rows");
    const paletteAddBtn = document.getElementById("ascii-palette-add");
    const previewPane = document.getElementById("ascii-preview-pane");
    const gridOverlay = document.getElementById("ascii-grid-overlay");
    const timelineEl = document.getElementById("ascii-timeline");
    const frameSlider = document.getElementById("ascii-frame-slider");
    const frameCounter = document.getElementById("ascii-frame-counter");
    const tlFirst = document.getElementById("ascii-tl-first");
    const tlPrev = document.getElementById("ascii-tl-prev");
    const tlPlay = document.getElementById("ascii-tl-play");
    const tlNext = document.getElementById("ascii-tl-next");
    const tlLast = document.getElementById("ascii-tl-last");
    const speedSelect = document.getElementById("ascii-speed-select");
    const opDup = document.getElementById("ascii-op-dup");
    const opBlank = document.getElementById("ascii-op-blank");
    const opDel = document.getElementById("ascii-op-del");
    const opLeft = document.getElementById("ascii-op-left");
    const opRight = document.getElementById("ascii-op-right");
    const toolsEl = document.getElementById("ascii-tools"); // the tool strip
    const toolChar = document.getElementById("ascii-tool-char");
    const toolClass = document.getElementById("ascii-tool-class");
    const asciiToolCursorBtn = document.getElementById("ascii-tool-cursor");
    const asciiToolPencilBtn = document.getElementById("ascii-tool-pencil");
    const asciiToolOnionBtn = document.getElementById("ascii-tool-onion");
    const asciiToolFlyout = document.getElementById("ascii-tool-flyout");
    const asciiFlyoutChar = document.getElementById("ascii-flyout-char");
    const asciiSwatchRow = document.getElementById("ascii-swatch-row");
    const asciiToolReplaceBtn = document.getElementById("ascii-tool-replace");
    const replaceBarEl = document.getElementById("ascii-replace-bar");
    const replaceFromInput = document.getElementById("ascii-replace-from");
    const replaceToInput = document.getElementById("ascii-replace-to");
    const replaceFromPreview = document.getElementById("ascii-replace-from-preview");
    const replaceToPreview = document.getElementById("ascii-replace-to-preview");
    const replaceApplyBtn = document.getElementById("ascii-replace-apply");
    const replaceHintEl = document.getElementById("ascii-replace-hint");
    const resizeHandle = document.getElementById("ascii-modal-resize-handle");
    const sidebar = modal?.querySelector(".ascii-modal-sidebar");
    const fitBtn = document.getElementById("ascii-fit-btn");
    const fitSuggestionsEl = document.getElementById("ascii-fit-suggestions");

    let modalMode = "create";
    let modalName = "";
    // ICG data model: frames are {chars: string, colors: Uint8Array}
    let localFrames = [];
    let localClassTable = [""]; // index 0 = "" = default color
    let confirmedCols = 80;
    let confirmedRows = 24;

    // ── Timeline state ──────────────────────────────────
    let currentFrameIndex = 0;
    let isPlaying = false;
    let playbackSpeed = 1;
    let playRafId = null;
    let lastPlayTimestamp = 0;
    let previewRo = null; // ResizeObserver for font scaling

    // ── Grid editing state ──────────────────────────────
    let isPainting = false;
    let gridRenderTimer = null;
    let onionSkinEnabled = false;
    let activeTool = "cursor"; // 'cursor' | 'pencil'
    let replaceScope = "all"; // 'all' | 'current'

    // ── ICG frame helpers ───────────────────────────────
    function makeBlankICGFrame(cols, rows) {
      return {
        chars: Array.from({ length: rows }, () => " ".repeat(cols)).join("\n"),
        colors: new Uint8Array(cols * rows),
      };
    }

    function getClassIndex(cls) {
      if (!cls) return 0;
      let idx = localClassTable.indexOf(cls);
      if (idx === -1) {
        localClassTable.push(cls);
        idx = localClassTable.length - 1;
      }
      return idx;
    }

    // ── Drag resize handle ───────────────────────────────
    if (resizeHandle && sidebar && modal) {
      let isResizing = false,
        resizeStartX = 0,
        resizeStartW = 0;
      resizeHandle.addEventListener("mousedown", (e) => {
        isResizing = true;
        resizeStartX = e.clientX;
        resizeStartW = sidebar.getBoundingClientRect().width;
        resizeHandle.classList.add("resizing");
        document.body.style.cursor = "col-resize";
        document.body.style.userSelect = "none";
      });
      document.addEventListener("mousemove", (e) => {
        if (!isResizing) return;
        const w = Math.max(
          180,
          Math.min(520, resizeStartW + (e.clientX - resizeStartX)),
        );
        modal.querySelector(".ascii-modal-body").style.gridTemplateColumns =
          `${w}px 5px 1fr`;
      });
      document.addEventListener("mouseup", () => {
        if (!isResizing) return;
        isResizing = false;
        resizeHandle.classList.remove("resizing");
        document.body.style.cursor = "";
        document.body.style.userSelect = "";
      });
    }

    // ── Editor font scaling (DOM-based preview) ──────────
    function setupAsciiCanvasFontScaling(el, onResize) {
      const CHAR_W = 0.6;
      const CHAR_H = 1.0;
      let cachedCols = 0;
      let cachedRows = 0;
      let lastDataCols = "";
      let lastDataRows = "";
      const ro = new ResizeObserver((entries) => {
        for (const entry of entries) {
          const { width, height } = entry.contentRect;
          if (
            el.dataset.asciiCols !== lastDataCols ||
            el.dataset.asciiRows !== lastDataRows
          ) {
            lastDataCols = el.dataset.asciiCols;
            lastDataRows = el.dataset.asciiRows;
            cachedCols = parseInt(lastDataCols, 10) || 40;
            cachedRows = parseInt(lastDataRows, 10) || 20;
          }
          el.style.fontSize =
            Math.floor(
              Math.min(
                width / (cachedCols * CHAR_W),
                height / (cachedRows * CHAR_H),
              ),
            ) + "px";
          if (onResize) onResize();
        }
      });
      ro.observe(el);
      return ro;
    }

    // ── ICG cell accessors ──────────────────────────────

    function getCell(frame, cols, row, col) {
      const lines = frame.chars.split("\n");
      const runes = Array.from(lines[row] || "");
      const ch = col < runes.length ? runes[col] : " ";
      const ci = frame.colors[row * cols + col] || 0;
      const cls = ci < localClassTable.length ? localClassTable[ci] : "";
      return { ch, cls };
    }

    function setCell(frame, cols, row, col, ch, cls) {
      const lines = frame.chars.split("\n");
      while (lines.length <= row)
        lines.push(" ".repeat(cols));
      const runes = Array.from(lines[row]);
      while (runes.length <= col)
        runes.push(" ");
      runes[col] = ch;
      lines[row] = runes.join("");
      frame.chars = lines.join("\n");
      frame.colors[row * cols + col] = getClassIndex(cls);
    }

    // Convert an ICG frame to HTML for the <pre>-based preview rendering.
    function icgFrameToHtml(frame, cols, rows) {
      const lines = frame.chars.split("\n");
      const result = [];
      let colorIdx = 0;
      for (let r = 0; r < rows; r++) {
        const runes = r < lines.length ? Array.from(lines[r]) : [];
        let line = "";
        let i = 0;
        while (i < cols) {
          if (i < runes.length) {
            const ci = frame.colors[colorIdx + i] || 0;
            const cls = ci < localClassTable.length ? localClassTable[ci] : "";
            let j = i + 1;
            while (j < runes.length && j < cols) {
              const nci = frame.colors[colorIdx + j] || 0;
              const ncls = nci < localClassTable.length ? localClassTable[nci] : "";
              if (ncls !== cls) break;
              j++;
            }
            const chars = runes.slice(i, j).map((c) => {
              if (c === "&") return "&amp;";
              if (c === "<") return "&lt;";
              if (c === ">") return "&gt;";
              return c;
            }).join("");
            if (cls) {
              line += `<span class="${escapeHtml(cls)}">${chars}</span>`;
            } else {
              line += chars;
            }
            i = j;
          } else {
            // Pad row to exactly cols characters so the <pre> width is uniform
            // and the grid overlay cells align correctly with character positions.
            line += " ".repeat(cols - i);
            i = cols;
          }
        }
        result.push(line);
        colorIdx += runes.length;
      }
      return result.join("\n");
    }

    // ── Frame normalization ──────────────────────────────

    // Serialize localFrames to JSON for API calls.
    function serializeICGFrames() {
      return localFrames.map((f) => ({
        chars: f.chars,
        colors: btoa(String.fromCharCode(...f.colors)),
      }));
    }

    // Normalize all localFrames to exactly cols×rows via server-side processing.
    async function normalizeFramesAsync(cols, rows) {
      const res = await fetch("/api/ascii/normalize", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          class_table: localClassTable,
          frames: serializeICGFrames(),
          cols,
          rows,
        }),
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.error || "Normalization failed");
      }
      const data = await res.json();
      localClassTable = data.class_table || [""];
      localFrames = (data.frames || []).map((f) => ({
        chars: f.chars,
        colors: Uint8Array.from(atob(f.colors), (c) => c.charCodeAt(0)),
      }));
    }

    // Returns true if normalizing to newCols×newRows would truncate visible content.
    async function wouldTruncateAsync(newCols, newRows) {
      const res = await fetch("/api/ascii/would-truncate", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          class_table: localClassTable,
          frames: serializeICGFrames(),
          cols: newCols,
          rows: newRows,
        }),
      });
      if (!res.ok) return false;
      const data = await res.json();
      return data.truncates;
    }

    // ── Palette helpers ──────────────────────────────────

    const _colorCtx = (() => {
      const c = document.createElement("canvas");
      c.width = c.height = 1;
      return c.getContext("2d");
    })();

    function colorToHex(color) {
      if (!color) return "#000000";
      try {
        _colorCtx.fillStyle = "#000000";
        _colorCtx.fillStyle = color;
        const v = _colorCtx.fillStyle;
        if (/^#[0-9a-f]{6}$/i.test(v)) return v;
        const m = v.match(/^rgba?\s*\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)/i);
        if (m)
          return (
            "#" +
            [m[1], m[2], m[3]]
              .map((n) => parseInt(n, 10).toString(16).padStart(2, "0"))
              .join("")
          );
      } catch {}
      return "#000000";
    }

    // Returns { hex: '#rrggbb', alpha: 0-100 } from any supported color string.
    function parseColorParts(color) {
      if (!color) return { hex: "#000000", alpha: 100 };
      const hex8 = color.match(/^#([0-9a-f]{6})([0-9a-f]{2})$/i);
      if (hex8) {
        return {
          hex: "#" + hex8[1],
          alpha: Math.round((parseInt(hex8[2], 16) / 255) * 100),
        };
      }
      const rgba = color.match(
        /^rgba\s*\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*,\s*([0-9.]+)\s*\)$/i,
      );
      if (rgba) {
        const hex =
          "#" +
          [rgba[1], rgba[2], rgba[3]]
            .map((n) => parseInt(n, 10).toString(16).padStart(2, "0"))
            .join("");
        return { hex, alpha: Math.round(parseFloat(rgba[4]) * 100) };
      }
      return { hex: colorToHex(color), alpha: 100 };
    }

    function buildPaletteColor(hex, alphaPct) {
      if (alphaPct >= 100) return hex;
      const r = parseInt(hex.slice(1, 3), 16);
      const g = parseInt(hex.slice(3, 5), 16);
      const b = parseInt(hex.slice(5, 7), 16);
      return `rgba(${r}, ${g}, ${b}, ${(alphaPct / 100).toFixed(2)})`;
    }

    function addPaletteRow(cls, color) {
      const { hex, alpha } = parseColorParts(color);
      const row = document.createElement("div");
      row.className = "ascii-palette-row";
      row.innerHTML = `
        <input type="text" class="app-input ascii-palette-class" placeholder="class-name" value="${escapeHtml(cls || "")}">
        <input type="color" class="ascii-palette-color" value="${hex}">
        <input type="range" class="ascii-palette-alpha" min="0" max="100" value="${alpha}" title="Opacity: ${alpha}%">
        <button class="ascii-palette-remove" type="button" title="Remove">&times;</button>
      `;
      row
        .querySelector(".ascii-palette-remove")
        .addEventListener("click", () => {
          row.remove();
          rebuildToolClassSelect();
          if (localFrames.length > 0) renderFrameLocally(currentFrameIndex);
        });
      const alphaSlider = row.querySelector(".ascii-palette-alpha");
      if (alphaSlider)
        alphaSlider.addEventListener("input", () => {
          alphaSlider.title = `Opacity: ${alphaSlider.value}%`;
        });
      if (paletteRowsEl) paletteRowsEl.appendChild(row);
      rebuildToolClassSelect();
    }

    if (paletteAddBtn)
      paletteAddBtn.addEventListener("click", () =>
        addPaletteRow("", "#ffffff"),
      );

    function getPalette() {
      const palette = {};
      if (!paletteRowsEl) return palette;
      paletteRowsEl.querySelectorAll(".ascii-palette-row").forEach((row) => {
        const cls = row.querySelector(".ascii-palette-class")?.value.trim();
        const hex = row.querySelector(".ascii-palette-color")?.value || "#000000";
        const alphaInput = row.querySelector(".ascii-palette-alpha");
        const alphaParsed = alphaInput ? parseInt(alphaInput.value, 10) : NaN;
        const alpha = isNaN(alphaParsed) ? 100 : Math.max(0, Math.min(100, alphaParsed));
        if (cls) palette[cls] = buildPaletteColor(hex, alpha);
      });
      return palette;
    }

    function rebuildToolClassSelect() {
      if (!toolClass) return;
      const current = toolClass.value;
      toolClass.innerHTML = '<option value="">(none)</option>';
      if (paletteRowsEl) {
        paletteRowsEl.querySelectorAll(".ascii-palette-row").forEach((row) => {
          const cls = row.querySelector(".ascii-palette-class")?.value.trim();
          if (cls) {
            const opt = document.createElement("option");
            opt.value = cls;
            opt.textContent = cls;
            if (cls === current) opt.selected = true;
            toolClass.appendChild(opt);
          }
        });
      }
      rebuildSwatches();
    }

    function rebuildSwatches() {
      if (!asciiSwatchRow) return;
      asciiSwatchRow.innerHTML = "";
      const currentCls = toolClass?.value || "";
      if (paletteRowsEl) {
        paletteRowsEl.querySelectorAll(".ascii-palette-row").forEach((row) => {
          const cls = row.querySelector(".ascii-palette-class")?.value.trim();
          const hex = row.querySelector(".ascii-palette-color")?.value || "#ffffff";
          const alphaInput = row.querySelector(".ascii-palette-alpha");
          const alphaParsed = alphaInput ? parseInt(alphaInput.value, 10) : NaN;
          const alpha = isNaN(alphaParsed) ? 100 : Math.max(0, Math.min(100, alphaParsed));
          const color = buildPaletteColor(hex, alpha);
          if (!cls) return;
          const btn = document.createElement("button");
          btn.className = "ascii-swatch";
          btn.type = "button";
          btn.title = cls;
          btn.style.setProperty("--swatch-color", color);
          btn.classList.toggle("active", currentCls === cls);
          btn.addEventListener("click", () => {
            if (toolClass) toolClass.value = cls;
            rebuildSwatches();
          });
          asciiSwatchRow.appendChild(btn);
        });
      }
    }

    // ── Client-side preview rendering ────────────────────

    function buildPaletteStyle(palette, scopeId) {
      const rules = Object.entries(palette)
        .map(
          ([cls, color]) =>
            `#${scopeId} .${CSS.escape(cls)} { color: ${color}; }`,
        )
        .join("\n");
      return rules;
    }

    function renderFrameLocally(frameIndex) {
      if (!previewPane || localFrames.length === 0) return;
      const cols = parseInt(colsInput?.value, 10) || 80;
      const rows = parseInt(rowsInput?.value, 10) || 24;
      const palette = getPalette();
      const scopeId = "ascii-preview-scope";

      // Remove old style
      const oldStyle = previewPane.querySelector(".ascii-preview-style");
      if (oldStyle) oldStyle.remove();

      let canvas = previewPane.querySelector(".ascii-canvas");
      if (!canvas) {
        // Build the full preview structure
        previewPane.innerHTML = "";
        const styleEl = document.createElement("style");
        styleEl.className = "ascii-preview-style";
        styleEl.textContent = buildPaletteStyle(palette, scopeId);
        previewPane.appendChild(styleEl);

        const wrapper = document.createElement("div");
        wrapper.id = scopeId;
        wrapper.className = "ascii-canvas widget-fill";
        wrapper.dataset.asciiCols = cols;
        wrapper.dataset.asciiRows = rows;
        previewPane.appendChild(wrapper);
        canvas = wrapper;

        // Re-append grid overlay
        if (gridOverlay) previewPane.appendChild(gridOverlay);

        // Font scaling ResizeObserver — re-aligns grid overlay after font is set
        if (previewRo) previewRo.disconnect();
        previewRo = setupAsciiCanvasFontScaling(canvas, () => {
          if (activeTool === "pencil") alignGridOverlay();
        });
      } else {
        // Update style block + dataset
        const styleEl = document.createElement("style");
        styleEl.className = "ascii-preview-style";
        styleEl.textContent = buildPaletteStyle(palette, scopeId);
        previewPane.insertBefore(styleEl, canvas);
        canvas.dataset.asciiCols = cols;
        canvas.dataset.asciiRows = rows;
        // Manually recalculate font size — ResizeObserver only fires on element resize,
        // not on dataset change, so we need to recompute when cols/rows change.
        const CHAR_W = 0.6,
          CHAR_H = 1.0;
        const { width, height } = canvas.getBoundingClientRect();
        if (width && height) {
          const fw = width / (cols * CHAR_W);
          const fh = height / (rows * CHAR_H);
          canvas.style.fontSize = Math.floor(Math.min(fw, fh)) + "px";
          if (activeTool === "pencil") alignGridOverlay();
        }
      }

      // Onion skin
      const oldOnion = canvas.querySelector(".ascii-onion");
      if (oldOnion) oldOnion.remove();
      if (onionSkinEnabled && frameIndex > 0) {
        const onion = document.createElement("pre");
        onion.className = "ascii-frame ascii-onion";
        onion.innerHTML = icgFrameToHtml(localFrames[frameIndex - 1], cols, rows);
        canvas.insertBefore(onion, canvas.firstChild);
      }

      // Current frame
      let pre = canvas.querySelector(".ascii-frame:not(.ascii-onion)");
      if (!pre) {
        pre = document.createElement("pre");
        pre.className = "ascii-frame";
        canvas.appendChild(pre);
      }
      pre.innerHTML = icgFrameToHtml(localFrames[frameIndex], cols, rows);

      updatePreviewAspectRatio();
      updateFrameCounter();
    }

    function updatePreviewAspectRatio() {
      // Aspect ratio is determined by the dashboard widget container shape, not the
      // character grid. size="WxH" maps to W grid-columns × H grid-rows.
      // Dashboard: max-width 900px container, padding 1.5rem each side, 5 cols,
      // gap 0.75rem (12px), row height 130px.
      const GRID_COL_W = 161; // (900 - 48 - 4*12) / 5
      const GRID_ROW_H = 130;
      const GRID_GAP = 12;
      const sizeVal = sizeInput?.value.trim() || "";
      const m = sizeVal.match(/^(\d+)x(\d+)$/i);
      if (m) {
        const W = parseInt(m[1], 10);
        const H = parseInt(m[2], 10);
        const pxW = W * GRID_COL_W + (W - 1) * GRID_GAP;
        const pxH = H * GRID_ROW_H + (H - 1) * GRID_GAP;
        if (previewPane) previewPane.style.aspectRatio = (pxW / pxH).toFixed(3);
      }
      // If size isn't filled in yet, leave aspect-ratio unchanged.
    }

    // ── Timeline ─────────────────────────────────────────

    function updateFrameCounter() {
      if (frameCounter)
        frameCounter.textContent = `${currentFrameIndex + 1} / ${localFrames.length}`;
      if (frameSlider) {
        frameSlider.max = Math.max(0, localFrames.length - 1);
        frameSlider.value = currentFrameIndex;
        const maxF = Math.max(1, localFrames.length - 1);
        frameSlider.style.setProperty(
          "--pct",
          `${(currentFrameIndex / maxF) * 100}%`,
        );
      }
      if (opDel) opDel.disabled = localFrames.length <= 1;
      if (opLeft) opLeft.disabled = currentFrameIndex === 0;
      if (opRight)
        opRight.disabled = currentFrameIndex >= localFrames.length - 1;
    }

    function syncTimeline() {
      updateFrameCounter();
      renderFrameLocally(currentFrameIndex);
    }

    function goToFrame(n) {
      currentFrameIndex = Math.max(0, Math.min(n, localFrames.length - 1));
      renderFrameLocally(currentFrameIndex);
      updateFrameCounter();
      if (activeTool === "pencil") {
        buildGridOverlay(
          parseInt(colsInput?.value, 10) || 80,
          parseInt(rowsInput?.value, 10) || 24,
        );
      }
      if (replaceScope === "current") updateReplaceBar();
    }

    function showTimeline() {
      if (timelineEl) timelineEl.style.display = "";
      if (toolsEl) toolsEl.style.display = "";
      if (modal) modal.classList.add("editor-active");
      rebuildToolClassSelect();
    }

    function playTick(timestamp) {
      if (!isPlaying) return;
      const fps = parseInt(fpsInput?.value, 10) || 12;
      const interval = 1000 / (fps * playbackSpeed);
      if (timestamp - lastPlayTimestamp >= interval) {
        lastPlayTimestamp = timestamp;
        const next = (currentFrameIndex + 1) % localFrames.length;
        goToFrame(next);
      }
      playRafId = requestAnimationFrame(playTick);
    }

    function togglePlay() {
      if (localFrames.length === 0) return;
      isPlaying = !isPlaying;
      if (isPlaying) {
        lastPlayTimestamp = 0;
        if (tlPlay) {
          tlPlay.textContent = "⏸";
          tlPlay.classList.add("playing");
        }
        playRafId = requestAnimationFrame(playTick);
      } else {
        if (playRafId) cancelAnimationFrame(playRafId);
        if (tlPlay) {
          tlPlay.innerHTML = "&#x25B6;";
          tlPlay.classList.remove("playing");
        }
      }
    }

    if (tlFirst) tlFirst.addEventListener("click", () => goToFrame(0));
    if (tlPrev)
      tlPrev.addEventListener("click", () => goToFrame(currentFrameIndex - 1));
    if (tlPlay) tlPlay.addEventListener("click", togglePlay);
    if (tlNext)
      tlNext.addEventListener("click", () => goToFrame(currentFrameIndex + 1));
    if (tlLast)
      tlLast.addEventListener("click", () => goToFrame(localFrames.length - 1));
    if (frameSlider)
      frameSlider.addEventListener("input", () => {
        const val = parseInt(frameSlider.value, 10);
        const maxF = Math.max(1, parseInt(frameSlider.max, 10));
        frameSlider.style.setProperty("--pct", `${(val / maxF) * 100}%`);
        goToFrame(val);
      });
    if (speedSelect)
      speedSelect.addEventListener("change", () => {
        playbackSpeed = parseFloat(speedSelect.value);
      });

    // Keyboard shortcuts (when modal open and not in a text input)
    document.addEventListener("keydown", (e) => {
      if (!overlay.classList.contains("open")) return;
      if (localFrames.length === 0) return;
      const tag = document.activeElement?.tagName;
      if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return;
      if (e.key === "ArrowLeft") {
        e.preventDefault();
        goToFrame(currentFrameIndex - 1);
      } else if (e.key === "ArrowRight") {
        e.preventDefault();
        goToFrame(currentFrameIndex + 1);
      } else if (e.key === " ") {
        e.preventDefault();
        togglePlay();
      } else if (e.key === "Home") {
        e.preventDefault();
        goToFrame(0);
      } else if (e.key === "End") {
        e.preventDefault();
        goToFrame(localFrames.length - 1);
      }
    });

    // ── Frame operations ─────────────────────────────────

    if (opDup)
      opDup.addEventListener("click", () => {
        if (localFrames.length === 0) return;
        const src = localFrames[currentFrameIndex];
        localFrames.splice(currentFrameIndex + 1, 0, {
          chars: src.chars,
          colors: new Uint8Array(src.colors),
        });
        goToFrame(currentFrameIndex + 1);
        syncTimeline();
      });

    if (opBlank)
      opBlank.addEventListener("click", () => {
        const cols = parseInt(colsInput?.value, 10) || 80;
        const rows = parseInt(rowsInput?.value, 10) || 24;
        localFrames.splice(currentFrameIndex + 1, 0, makeBlankICGFrame(cols, rows));
        goToFrame(currentFrameIndex + 1);
        syncTimeline();
      });

    if (opDel)
      opDel.addEventListener("click", () => {
        if (localFrames.length <= 1) return;
        localFrames.splice(currentFrameIndex, 1);
        goToFrame(Math.min(currentFrameIndex, localFrames.length - 1));
        syncTimeline();
      });

    if (opLeft)
      opLeft.addEventListener("click", () => {
        if (currentFrameIndex === 0) return;
        [localFrames[currentFrameIndex], localFrames[currentFrameIndex - 1]] = [
          localFrames[currentFrameIndex - 1],
          localFrames[currentFrameIndex],
        ];
        goToFrame(currentFrameIndex - 1);
        syncTimeline();
      });

    if (opRight)
      opRight.addEventListener("click", () => {
        if (currentFrameIndex >= localFrames.length - 1) return;
        [localFrames[currentFrameIndex], localFrames[currentFrameIndex + 1]] = [
          localFrames[currentFrameIndex + 1],
          localFrames[currentFrameIndex],
        ];
        goToFrame(currentFrameIndex + 1);
        syncTimeline();
      });

    // ── Grid overlay + painting ───────────────────────────

    // Position the grid overlay to exactly cover the rendered <pre> element.
    // Called after buildGridOverlay and whenever the pane resizes.
    function alignGridOverlay() {
      if (!gridOverlay || !previewPane) return;
      const canvas = previewPane.querySelector(".ascii-canvas");
      if (!canvas) return;
      // Bail if font scaling hasn't fired yet — the onResize callback will
      // re-invoke this function once canvas.style.fontSize is set.
      if (!parseFloat(canvas.style.fontSize)) return;
      const pre = canvas.querySelector(".ascii-frame:not(.ascii-onion)");
      if (!pre) return;
      const paneRect = previewPane.getBoundingClientRect();
      const preRect = pre.getBoundingClientRect();
      const paneStyle = getComputedStyle(previewPane);
      const borderLeft = parseFloat(paneStyle.borderLeftWidth) || 0;
      const borderTop = parseFloat(paneStyle.borderTopWidth) || 0;
      gridOverlay.style.left = preRect.left - paneRect.left - borderLeft + "px";
      gridOverlay.style.top = preRect.top - paneRect.top - borderTop + "px";
      gridOverlay.style.width = preRect.width + "px";
      gridOverlay.style.height = preRect.height + "px";
    }

    // Re-align whenever the preview pane itself changes size (window resize, sidebar drag, etc.)
    if (previewPane) {
      new ResizeObserver(() => {
        if (activeTool === "pencil") alignGridOverlay();
      }).observe(previewPane);
    }

    function buildGridOverlay(cols, rows) {
      if (!gridOverlay) return;
      gridOverlay.style.display = "grid";
      gridOverlay.style.gridTemplateColumns = `repeat(${cols}, 1fr)`;
      gridOverlay.style.gridTemplateRows = `repeat(${rows}, 1fr)`;
      gridOverlay.innerHTML = "";
      for (let r = 0; r < rows; r++) {
        for (let c = 0; c < cols; c++) {
          const cell = document.createElement("div");
          cell.className = "ascii-grid-cell";
          cell.dataset.row = r;
          cell.dataset.col = c;
          gridOverlay.appendChild(cell);
        }
      }
      // Align after the browser has laid out the new font size / frame content.
      requestAnimationFrame(alignGridOverlay);
    }

    // Convert a pointer event to (row, col) using floating-point division of the
    // overlay rect. This avoids sub-pixel drift from CSS grid 1fr column rounding
    // at small font sizes (e.g. 1.2px/cell at fontSize=2px).
    function cellFromEvent(e) {
      const rect = gridOverlay.getBoundingClientRect();
      const cols = parseInt(colsInput?.value, 10) || 80;
      const rows = parseInt(rowsInput?.value, 10) || 24;
      const c = Math.floor((e.clientX - rect.left) / rect.width * cols);
      const r = Math.floor((e.clientY - rect.top) / rect.height * rows);
      if (c < 0 || c >= cols || r < 0 || r >= rows) return null;
      return { r, c };
    }

    function paintCell(r, c) {
      const cols = parseInt(colsInput?.value, 10) || 80;
      const ch = Array.from(toolChar?.value || " ")[0] || " ";
      const cls = toolClass?.value || "";
      setCell(localFrames[currentFrameIndex], cols, r, c, ch, cls);
      scheduleGridRender();
    }

    function scheduleGridRender() {
      if (gridRenderTimer) return;
      gridRenderTimer = setTimeout(() => {
        gridRenderTimer = null;
        const cols = parseInt(colsInput?.value, 10) || 80;
        const rows = parseInt(rowsInput?.value, 10) || 24;
        const canvas = previewPane?.querySelector(".ascii-canvas");
        const pre = canvas?.querySelector(".ascii-frame:not(.ascii-onion)");
        if (pre)
          pre.innerHTML = icgFrameToHtml(
            localFrames[currentFrameIndex],
            cols,
            rows,
          );
      }, 16);
    }

    if (gridOverlay) {
      gridOverlay.addEventListener("mousedown", (e) => {
        const cell = cellFromEvent(e);
        if (!cell) return;
        if (e.button === 2 || e.altKey) {
          // Eyedropper
          const cols = parseInt(colsInput?.value, 10) || 80;
          const picked = getCell(localFrames[currentFrameIndex], cols, cell.r, cell.c);
          if (picked) {
            if (toolChar) toolChar.value = picked.ch === " " ? " " : picked.ch;
            if (asciiFlyoutChar)
              asciiFlyoutChar.textContent = toolChar?.value || " ";
            if (toolClass) toolClass.value = picked.cls || "";
            rebuildSwatches();
          }
          return;
        }
        isPainting = true;
        paintCell(cell.r, cell.c);
      });
      gridOverlay.addEventListener("mousemove", (e) => {
        if (!isPainting) return;
        const cell = cellFromEvent(e);
        if (cell) paintCell(cell.r, cell.c);
      });
      gridOverlay.addEventListener("contextmenu", (e) => e.preventDefault());
    }
    document.addEventListener("mouseup", () => {
      isPainting = false;
    });

    function setActiveTool(tool) {
      activeTool = tool;
      const isPencil = tool === "pencil";
      if (asciiToolCursorBtn)
        asciiToolCursorBtn.classList.toggle("active", !isPencil);
      if (asciiToolPencilBtn)
        asciiToolPencilBtn.classList.toggle("active", isPencil);
      if (asciiToolFlyout)
        asciiToolFlyout.style.display = isPencil ? "" : "none";
      if (isPencil) {
        buildGridOverlay(
          parseInt(colsInput?.value, 10) || 80,
          parseInt(rowsInput?.value, 10) || 24,
        );
      } else {
        if (gridOverlay) {
          gridOverlay.style.display = "none";
          gridOverlay.innerHTML = "";
        }
      }
    }

    if (asciiToolCursorBtn)
      asciiToolCursorBtn.addEventListener("click", () =>
        setActiveTool("cursor"),
      );
    if (asciiToolPencilBtn)
      asciiToolPencilBtn.addEventListener("click", () => {
        if (activeTool === "pencil") {
          // Already in pencil mode — toggle the picker flyout without leaving draw mode
          if (asciiToolFlyout)
            asciiToolFlyout.style.display =
              asciiToolFlyout.style.display === "none" ? "" : "none";
        } else {
          setActiveTool("pencil");
        }
      });

    if (asciiToolOnionBtn)
      asciiToolOnionBtn.addEventListener("click", () => {
        onionSkinEnabled = !onionSkinEnabled;
        asciiToolOnionBtn.classList.toggle("active", onionSkinEnabled);
        if (localFrames.length > 0) renderFrameLocally(currentFrameIndex);
      });

    // ── Find & Replace bar ────────────────────────────────

    function getReplaceFromChar() {
      return Array.from(replaceFromInput?.value || "")[0] ?? null;
    }

    function getReplaceToChar() {
      const val = replaceToInput?.value ?? "";
      if (val === "") return " "; // empty = erase (replace with space)
      return Array.from(val)[0] ?? " ";
    }

    function countReplaceMatches(fromChar) {
      if (!fromChar || localFrames.length === 0) return 0;
      const frames =
        replaceScope === "current"
          ? [localFrames[currentFrameIndex]]
          : localFrames;
      let count = 0;
      for (const frame of frames) {
        for (const ch of frame.chars) {
          if (ch === fromChar) count++;
        }
      }
      return count;
    }

    function updateReplaceBar() {
      if (!replaceBarEl || replaceBarEl.style.display === "none") return;
      const fromChar = getReplaceFromChar();
      const hasFrom = fromChar !== null && fromChar !== "";

      if (replaceFromPreview) {
        replaceFromPreview.textContent = hasFrom ? fromChar : "·";
        replaceFromPreview.classList.toggle("empty", !hasFrom);
      }

      const toVal = replaceToInput?.value ?? "";
      const hasTo = toVal !== "";
      if (replaceToPreview) {
        replaceToPreview.textContent = hasTo
          ? Array.from(toVal)[0] ?? "·"
          : "·";
        replaceToPreview.classList.toggle("empty", !hasTo);
      }

      if (replaceHintEl) {
        replaceHintEl.className = "ascii-replace-hint";
        replaceHintEl.textContent = "";
      }

      if (!hasFrom) {
        if (replaceApplyBtn) replaceApplyBtn.disabled = true;
        return;
      }

      const count = countReplaceMatches(fromChar);
      if (replaceApplyBtn) replaceApplyBtn.disabled = count === 0;

      if (replaceHintEl) {
        if (count === 0) {
          replaceHintEl.textContent = "no matches";
        } else {
          const n = localFrames.length;
          const scopeLabel =
            replaceScope === "current"
              ? "in this frame"
              : `across ${n} frame${n === 1 ? "" : "s"}`;
          replaceHintEl.textContent = `${count} match${count === 1 ? "" : "es"} ${scopeLabel}`;
        }
      }
    }

    function openReplaceBar() {
      if (!replaceBarEl) return;
      replaceBarEl.style.display = "";
      if (asciiToolReplaceBtn) asciiToolReplaceBtn.classList.add("active");
      updateReplaceBar();
      replaceFromPreview?.focus();
      replaceFromInput?.focus();
    }

    function closeReplaceBar() {
      if (!replaceBarEl) return;
      replaceBarEl.style.display = "none";
      if (asciiToolReplaceBtn) asciiToolReplaceBtn.classList.remove("active");
    }

    if (asciiToolReplaceBtn) {
      asciiToolReplaceBtn.addEventListener("click", () => {
        const isOpen = replaceBarEl?.style.display !== "none";
        if (isOpen) {
          closeReplaceBar();
        } else {
          openReplaceBar();
        }
      });
    }

    // Click on char preview box focuses the hidden backing input
    [
      [replaceFromPreview, replaceFromInput],
      [replaceToPreview, replaceToInput],
    ].forEach(([preview, input]) => {
      if (preview && input)
        preview.addEventListener("click", () => {
          input.focus();
          input.select();
        });
    });

    // Inputs: clamp to 1 codepoint, sync preview
    [replaceFromInput, replaceToInput].forEach((input) => {
      if (!input) return;
      input.addEventListener("focus", () => input.select());
      input.addEventListener("input", () => {
        const cps = Array.from(input.value);
        if (cps.length > 1) input.value = cps[cps.length - 1];
        updateReplaceBar();
      });
    });

    // Scope toggle buttons
    document.querySelectorAll(".ascii-replace-scope-btn").forEach((btn) => {
      btn.addEventListener("click", () => {
        document
          .querySelectorAll(".ascii-replace-scope-btn")
          .forEach((b) => b.classList.remove("active"));
        btn.classList.add("active");
        replaceScope = btn.dataset.scope;
        updateReplaceBar();
      });
    });

    // Apply replacement
    if (replaceApplyBtn) {
      replaceApplyBtn.addEventListener("click", async () => {
        const fromChar = getReplaceFromChar();
        if (!fromChar) return;
        const toChar = getReplaceToChar();

        replaceApplyBtn.disabled = true;
        replaceApplyBtn.textContent = "…";
        if (replaceHintEl) {
          replaceHintEl.textContent = "";
          replaceHintEl.className = "ascii-replace-hint";
        }

        try {
          const frameIndex =
            replaceScope === "current" ? currentFrameIndex : null;
          const res = await fetch("/api/ascii/replace-char", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
              from: fromChar,
              to: toChar,
              class_table: localClassTable,
              frames: serializeICGFrames(),
              frame_index: frameIndex,
            }),
          });

          if (!res.ok) {
            const data = await res.json().catch(() => ({}));
            throw new Error(data.error || "Remap failed");
          }

          const data = await res.json();
          const updated = data.frames || [];
          updated.forEach((f, i) => {
            if (i < localFrames.length) {
              localFrames[i] = {
                chars: f.chars,
                colors: Uint8Array.from(atob(f.colors), (c) =>
                  c.charCodeAt(0),
                ),
              };
            }
          });

          renderFrameLocally(currentFrameIndex);

          const n = data.count || 0;
          if (replaceHintEl) {
            replaceHintEl.textContent =
              n === 0
                ? "nothing remapped"
                : `remapped ${n} occurrence${n === 1 ? "" : "s"}`;
            replaceHintEl.classList.toggle("success", n > 0);
          }

          // After 2s, refresh the match count
          setTimeout(() => updateReplaceBar(), 2000);
        } catch (err) {
          if (errorsEl) errorsEl.textContent = err.message;
          updateReplaceBar();
        } finally {
          replaceApplyBtn.textContent = "Remap";
          // Re-evaluate disabled state
          const from = getReplaceFromChar();
          replaceApplyBtn.disabled =
            !from || countReplaceMatches(from) === 0;
        }
      });
    }

    // Flyout char display: click to focus hidden input
    if (asciiFlyoutChar)
      asciiFlyoutChar.addEventListener("click", () => toolChar?.focus());

    if (toolChar) {
      // Auto-select on focus so typing replaces the existing char rather than appending.
      toolChar.addEventListener("focus", () => toolChar.select());
      // Clamp to 1 codepoint after each keystroke; sync display span.
      toolChar.addEventListener("input", () => {
        const codepoints = Array.from(toolChar.value);
        if (codepoints.length > 1)
          toolChar.value = codepoints[codepoints.length - 1];
        if (asciiFlyoutChar)
          asciiFlyoutChar.textContent = toolChar.value || " ";
      });
    }

    // ── Frame loading helper ──────────────────────────────

    function framesLoaded() {
      if (localFrames.length === 0) return;
      const cols = parseInt(colsInput?.value, 10) || 80;
      const rows = parseInt(rowsInput?.value, 10) || 24;
      confirmedCols = cols;
      confirmedRows = rows;
      currentFrameIndex = 0;
      showTimeline();
      syncTimeline();
    }

    // Palette changes → re-render palette style only
    if (paletteRowsEl)
      paletteRowsEl.addEventListener("input", () => {
        rebuildToolClassSelect();
        if (localFrames.length > 0) renderFrameLocally(currentFrameIndex);
      });

    // Size change → update aspect ratio (size drives the container shape)
    if (sizeInput)
      sizeInput.addEventListener("input", () => {
        updatePreviewAspectRatio();
        if (fitSuggestionsEl) fitSuggestionsEl.hidden = true;
      });

    // ── Fit to widget ────────────────────────────────────

    function computeFitSuggestions() {
      const GRID_COL_W = 161,
        GRID_ROW_H = 130,
        GRID_GAP = 12,
        CHAR_W = 0.6;
      const m = sizeInput?.value.trim().match(/^(\d+)x(\d+)$/i);
      if (!m) return [];
      const pxW =
        parseInt(m[1], 10) * GRID_COL_W + (parseInt(m[1], 10) - 1) * GRID_GAP;
      const pxH =
        parseInt(m[2], 10) * GRID_ROW_H + (parseInt(m[2], 10) - 1) * GRID_GAP;
      const k = pxW / pxH / CHAR_W; // target cols/rows ratio
      const C = parseInt(colsInput?.value, 10) || 80;
      const R = parseInt(rowsInput?.value, 10) || 24;

      const rowsB = Math.max(1, Math.round((k * C + R) / (k * k + 1)));
      const suggestions = [
        { label: "Keep rows", cols: Math.max(1, Math.round(R * k)), rows: R },
        { label: "Keep cols", cols: C, rows: Math.max(1, Math.round(C / k)) },
        {
          label: "Balanced",
          cols: Math.max(1, Math.round(k * rowsB)),
          rows: rowsB,
        },
      ];

      // Deduplicate by cols×rows
      const seen = new Set();
      return suggestions.filter((s) => {
        const key = `${s.cols}x${s.rows}`;
        return seen.has(key) ? false : (seen.add(key), true);
      });
    }

    if (fitBtn)
      fitBtn.addEventListener("click", () => {
        if (!fitSuggestionsEl) return;
        if (!fitSuggestionsEl.hidden) {
          fitSuggestionsEl.hidden = true;
          return;
        }
        const suggestions = computeFitSuggestions();
        if (suggestions.length === 0) return;
        fitSuggestionsEl.innerHTML = "";
        suggestions.forEach((s) => {
          const btn = document.createElement("button");
          btn.type = "button";
          btn.className = "ascii-fit-suggestion";
          btn.innerHTML = `<span class="ascii-fit-label">${s.label}</span><span class="ascii-fit-value">${s.cols} × ${s.rows}</span>`;
          btn.addEventListener("click", () => {
            fitSuggestionsEl.hidden = true;
            if (colsInput) colsInput.value = s.cols;
            if (rowsInput) rowsInput.value = s.rows;
            colsInput?.dispatchEvent(new Event("input"));
            colsInput?.dispatchEvent(new Event("change"));
            rowsInput?.dispatchEvent(new Event("change"));
          });
          fitSuggestionsEl.appendChild(btn);
        });
        fitSuggestionsEl.hidden = false;
      });

    // Close fit suggestions when clicking outside
    document.addEventListener("click", (e) => {
      if (
        fitSuggestionsEl &&
        !fitSuggestionsEl.hidden &&
        !fitBtn?.contains(e.target) &&
        !fitSuggestionsEl.contains(e.target)
      ) {
        fitSuggestionsEl.hidden = true;
      }
    });

    // Cols/rows change → live re-render on input, normalize+confirm on commit
    [colsInput, rowsInput].forEach((el) => {
      if (!el) return;
      el.addEventListener("input", () => {
        if (localFrames.length > 0) renderFrameLocally(currentFrameIndex);
        if (activeTool === "pencil") {
          buildGridOverlay(
            parseInt(colsInput?.value, 10) || 80,
            parseInt(rowsInput?.value, 10) || 24,
          );
        }
      });
      el.addEventListener("change", () => {
        const newCols = parseInt(colsInput?.value, 10) || 80;
        const newRows = parseInt(rowsInput?.value, 10) || 24;
        if (localFrames.length === 0) {
          confirmedCols = newCols;
          confirmedRows = newRows;
          return;
        }
        const applyResize = async () => {
          // Show loading state while server normalizes frames.
          if (previewPane) previewPane.classList.add("ascii-loading");
          try {
            await normalizeFramesAsync(newCols, newRows);
          } catch (err) {
            if (errorsEl)
              errorsEl.textContent = "Resize failed: " + err.message;
            if (previewPane) previewPane.classList.remove("ascii-loading");
            return;
          }
          if (previewPane) previewPane.classList.remove("ascii-loading");
          confirmedCols = newCols;
          confirmedRows = newRows;
          syncTimeline();
          if (activeTool === "pencil") buildGridOverlay(newCols, newRows);
        };
        const revertResize = () => {
          if (colsInput) colsInput.value = confirmedCols;
          if (rowsInput) rowsInput.value = confirmedRows;
          renderFrameLocally(currentFrameIndex);
          if (activeTool === "pencil")
            buildGridOverlay(confirmedCols, confirmedRows);
        };
        // Check server-side if resize would truncate content.
        if (previewPane) previewPane.classList.add('ascii-loading');
        wouldTruncateAsync(newCols, newRows).then(truncates => {
          if (previewPane) previewPane.classList.remove('ascii-loading');
          if (truncates) {
            showConfirm(
              `Resizing to ${newCols}×${newRows} will permanently trim content outside the new boundaries. Continue?`,
              applyResize,
              revertResize,
            );
          } else {
            applyResize();
          }
        });
      });
    });

    // ── Modal open/close ──────────────────────────────────

    function closeModal() {
      dismissConfirm();
      if (fitSuggestionsEl) fitSuggestionsEl.hidden = true;
      if (isPlaying) togglePlay();
      if (playRafId) cancelAnimationFrame(playRafId);
      if (previewRo) {
        previewRo.disconnect();
        previewRo = null;
      }
      activeTool = "cursor";
      closeReplaceBar();
      if (replaceFromInput) replaceFromInput.value = "";
      if (replaceToInput) replaceToInput.value = "";
      replaceScope = "all";
      document.querySelectorAll(".ascii-replace-scope-btn").forEach((b, i) => {
        b.classList.toggle("active", i === 0);
      });
      if (asciiToolFlyout) asciiToolFlyout.style.display = "none";
      overlay.classList.remove("open");
      if (modal) modal.classList.remove("editor-active");
      // Clear preview pane so injected <style> palette block is removed (CSS rules are global)
      if (previewPane) previewPane.innerHTML = "";
      // Re-append grid overlay (it lives inside previewPane but we just cleared it)
      if (gridOverlay) {
        gridOverlay.style.display = "none";
        gridOverlay.innerHTML = "";
        if (previewPane) previewPane.appendChild(gridOverlay);
      }
      localFrames = [];
      localClassTable = [""];
      currentFrameIndex = 0;
      isPlaying = false;
    }

    if (closeBtn) closeBtn.addEventListener("click", closeModal);
    if (cancelBtn) cancelBtn.addEventListener("click", closeModal);
    overlay.addEventListener("click", (e) => {
      if (e.target === overlay) closeModal();
    });
    document.addEventListener("keydown", (e) => {
      if (!overlay.classList.contains("open")) return;
      if (e.key === "Escape") {
        if (replaceBarEl && replaceBarEl.style.display !== "none") {
          closeReplaceBar();
        } else {
          closeModal();
        }
        return;
      }
      if ((e.metaKey || e.ctrlKey) && e.key === "h") {
        e.preventDefault();
        asciiToolReplaceBtn?.click();
      }
    });

    function resetModal() {
      if (errorsEl) errorsEl.textContent = "";
      if (previewPane) {
        previewPane.innerHTML =
          '<p class="ascii-preview-empty">Add a blank frame to begin</p>';
        if (gridOverlay) {
          gridOverlay.style.display = "none";
          gridOverlay.innerHTML = "";
          previewPane.appendChild(gridOverlay);
        }
      }
      if (paletteRowsEl) paletteRowsEl.innerHTML = "";
      if (timelineEl) timelineEl.style.display = "none";
      if (toolsEl) toolsEl.style.display = "none";
      if (modal) modal.classList.remove("editor-active");
      if (frameSlider) {
        frameSlider.max = 0;
        frameSlider.value = 0;
        frameSlider.style.setProperty("--pct", "0%");
      }
      if (frameCounter) frameCounter.textContent = "0 / 0";
      localFrames = [];
      localClassTable = [""];
      currentFrameIndex = 0;
      isPlaying = false;
      onionSkinEnabled = false;
      activeTool = "cursor";
      if (asciiToolCursorBtn) asciiToolCursorBtn.classList.remove("active");
      if (asciiToolPencilBtn) asciiToolPencilBtn.classList.remove("active");
      if (asciiToolOnionBtn) asciiToolOnionBtn.classList.remove("active");
      if (asciiToolFlyout) asciiToolFlyout.style.display = "none";
    }

    function openModal(mode, animName, variantSize) {
      modalMode = mode;
      modalName = animName || "";
      if (isPlaying) togglePlay();
      if (playRafId) cancelAnimationFrame(playRafId);
      if (previewRo) {
        previewRo.disconnect();
        previewRo = null;
      }
      resetModal();

      if (mode === "create") {
        if (titleEl) titleEl.textContent = "New Animation";
        if (nameInput) {
          nameInput.value = "";
          nameInput.disabled = false;
        }
        if (sizeInput) sizeInput.value = "";
        if (colsInput) colsInput.value = "80";
        if (rowsInput) rowsInput.value = "24";
        if (fpsInput) fpsInput.value = "12";
        updatePreviewAspectRatio();
        localClassTable = [""];
        localFrames = [makeBlankICGFrame(80, 24)];
        framesLoaded();
      } else {
        if (titleEl) titleEl.textContent = `Edit: ${animName}`;
        if (nameInput) {
          nameInput.value = animName;
          nameInput.disabled = true;
        }
        if (sizeInput) sizeInput.value = variantSize || "";
        if (colsInput) colsInput.value = "80";
        if (rowsInput) rowsInput.value = "24";
        if (fpsInput) fpsInput.value = "12";

        let metaPalette = {}; // palette from variant metadata (class->color)
        const metaP = fetch(
          `/api/ascii/animations/${encodeURIComponent(animName)}`,
        )
          .then((r) => r.json())
          .then((variants) => {
            const v = variantSize
              ? variants.find((vv) => vv.size === variantSize)
              : variants[0];
            if (!v) return;
            if (sizeInput) sizeInput.value = v.size || variantSize || "";
            if (colsInput) colsInput.value = v.cols || 80;
            if (rowsInput) rowsInput.value = v.rows || 24;
            if (fpsInput) fpsInput.value = v.fps || 12;
            if (v.palette) {
              metaPalette = v.palette;
              Object.entries(v.palette).forEach(([cls, color]) =>
                addPaletteRow(cls, color),
              );
            }
          })
          .catch(() => {});

        if (variantSize) {
          // Wait for metaP first so we have the palette for reverse-mapping
          metaP.then(() => {
            fetch(
              `/api/ascii/frames/${encodeURIComponent(animName)}?size=${encodeURIComponent(variantSize)}`,
            )
              .then((r) => r.json())
              .then((data) => {
                // ICG wire format: { palette, cols, rows, frames: [{chars, colors}] }
                if (data.palette && data.frames) {
                  // Rebuild localClassTable: reverse-map resolved wire colors
                  // back to class names using the variant's palette from meta.
                  const colorToClass = {};
                  for (const [cls, color] of Object.entries(metaPalette)) {
                    colorToClass[color] = cls;
                  }
                  localClassTable = [""];
                  for (let i = 1; i < data.palette.length; i++) {
                    const color = data.palette[i];
                    const cls = colorToClass[color] || `c${i}`;
                    localClassTable.push(cls);
                  }
                  localFrames = data.frames.map((f) => ({
                    chars: f.chars,
                    colors: Uint8Array.from(atob(f.colors), (c) => c.charCodeAt(0)),
                  }));
                  if (localFrames.length > 0) framesLoaded();
                }
              })
              .catch(() => {});
          });
        }
      }

      overlay.classList.add("open");
    }

    window.openAsciiModal = openModal;

    document.querySelectorAll(".ascii-new-btn").forEach((btn) => {
      btn.addEventListener("click", () => openModal("create"));
    });

    document.querySelectorAll(".ascii-edit-variant-btn").forEach((btn) => {
      btn.addEventListener("click", () =>
        openModal("edit", btn.dataset.name, btn.dataset.size),
      );
    });

    // ── Validation + Save ────────────────────────────────

    function validate() {
      const name = nameInput?.value.trim() || "";
      const size = sizeInput?.value.trim() || "";
      const cols = parseInt(colsInput?.value, 10);
      const rows = parseInt(rowsInput?.value, 10);
      const fps = parseInt(fpsInput?.value, 10);
      if (!name || !/^[a-zA-Z0-9_-]+$/.test(name))
        return "Name must be non-empty and contain only letters, numbers, hyphens, and underscores";
      if (!size || !/^\d+x\d+$/.test(size))
        return "Size must be in format NxN (e.g. 2x1)";
      if (!cols || cols < 1) return "Cols must be a positive integer";
      if (!rows || rows < 1) return "Rows must be a positive integer";
      if (!fps || fps < 1 || fps > 60) return "FPS must be between 1 and 60";
      if (localFrames.length === 0) return "At least one frame is required";
      return null;
    }

    if (saveBtn) {
      saveBtn.addEventListener("click", () => {
        const err = validate();
        if (err) {
          if (errorsEl) errorsEl.textContent = err;
          return;
        }
        if (errorsEl) errorsEl.textContent = "";

        const name = nameInput?.value.trim();
        const size = sizeInput?.value.trim();
        const cols = parseInt(colsInput?.value, 10);
        const rows = parseInt(rowsInput?.value, 10);
        const fps = parseInt(fpsInput?.value, 10);
        const palette = getPalette();
        const isCreate = modalMode === "create";
        const url = isCreate
          ? "/api/ascii/animations"
          : `/api/ascii/animations/${encodeURIComponent(modalName)}`;
        const method = isCreate ? "POST" : "PUT";

        saveBtn.disabled = true;
        saveBtn.textContent = "Saving…";

        fetch(url, {
          method,
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            name,
            size,
            cols,
            rows,
            fps,
            palette,
            class_table: localClassTable,
            icg_frames: serializeICGFrames(),
          }),
        })
          .then((r) => {
            if (r.ok || r.status === 201) return r.json();
            return r.json().then((data) => {
              throw new Error(data.error || "Save failed");
            });
          })
          .then(() => {
            closeModal();
            showToast(isCreate ? `Created "${name}"` : `Updated "${name}"`, {
              type: "success",
            });
            setTimeout(() => window.location.reload(), 400);
          })
          .catch((err) => {
            if (errorsEl) errorsEl.textContent = err.message;
            saveBtn.disabled = false;
            saveBtn.textContent = "Save";
          });
      });
    }
  }

  // ── Sortable tables with pagination ────────────────────
  function initSortable(table) {
    const PAGE_SIZE = 10;
    const tbody = table.tBodies[0];
    let sortedRows = [...tbody.rows];
    let currentPage = 0;
    let currentCol = null;
    let ascending = true;

    const defaultTh = table.querySelector("th[data-sort-default]");
    if (defaultTh) sort(defaultTh, false);
    else renderPage();

    if (sortedRows.length > PAGE_SIZE) {
      // wrap in div to lock height
      const wrapper = document.createElement("div");
      table.replaceWith(wrapper);
      wrapper.appendChild(table);

      injectControls(wrapper);

      document.fonts.ready.then(() => {
        wrapper.style.minHeight = wrapper.offsetHeight + "px";
      });
    }

    table.querySelectorAll("th[data-col]").forEach((th) => {
      th.addEventListener("click", () => {
        const isActive = th === currentCol;
        sort(th, isActive ? !ascending : true);
      });
    });

    function sort(th, asc) {
      const colIndex = [...th.parentElement.children].indexOf(th);
      sortedRows.sort((a, b) => {
        const av = a.cells[colIndex]?.dataset.val ?? "";
        const bv = b.cells[colIndex]?.dataset.val ?? "";
        const an = Number(av),
          bn = Number(bv);
        const cmp = !isNaN(an) && !isNaN(bn) ? an - bn : av.localeCompare(bv);
        return asc ? cmp : -cmp;
      });
      sortedRows.forEach((r) => tbody.appendChild(r));
      table
        .querySelectorAll("th[data-col]")
        .forEach((h) => h.removeAttribute("aria-sort"));
      th.setAttribute("aria-sort", asc ? "ascending" : "descending");
      currentCol = th;
      ascending = asc;
      currentPage = 0;
      renderPage();
      updateControls();
    }

    function renderPage() {
      const start = currentPage * PAGE_SIZE;
      const end = start + PAGE_SIZE;
      sortedRows.forEach((r, i) => {
        r.hidden = i < start || i >= end;
      });
    }

    function injectControls(anchor) {
      const container = document.createElement("div");
      container.className = "table-pagination";

      const prev = document.createElement("button");
      prev.className = "pagination-btn";
      prev.textContent = "←";
      prev.addEventListener("click", () => {
        if (currentPage > 0) {
          currentPage--;
          renderPage();
          updateControls();
        }
      });

      const pageNums = document.createElement("div");
      pageNums.className = "pagination-pages";

      const next = document.createElement("button");
      next.className = "pagination-btn";
      next.textContent = "→";
      next.addEventListener("click", () => {
        if (currentPage < Math.ceil(sortedRows.length / PAGE_SIZE) - 1) {
          currentPage++;
          renderPage();
          updateControls();
        }
      });

      const info = document.createElement("span");
      info.className = "pagination-info";

      container.appendChild(prev);
      container.appendChild(pageNums);
      container.appendChild(next);
      container.appendChild(info);
      anchor.insertAdjacentElement("afterend", container);

      table._pgPrev = prev;
      table._pgNext = next;
      table._pgPageNums = pageNums;
      table._pgInfo = info;
      updateControls();
    }

    function updateControls() {
      if (!table._pgInfo) return;
      const total = sortedRows.length;
      const totalPages = Math.ceil(total / PAGE_SIZE);
      const start = currentPage * PAGE_SIZE + 1;
      const end = Math.min(start + PAGE_SIZE - 1, total);

      table._pgInfo.textContent = `${start}–${end} of ${total}`;
      table._pgPrev.disabled = currentPage === 0;
      table._pgNext.disabled = currentPage >= totalPages - 1;

      // Rebuild page-number buttons with smart windowing.
      const pn = table._pgPageNums;
      pn.innerHTML = "";
      pageRange(currentPage, totalPages).forEach((p) => {
        if (p === "…") {
          const sep = document.createElement("span");
          sep.className = "pagination-ellipsis";
          sep.textContent = "…";
          pn.appendChild(sep);
        } else {
          const btn = document.createElement("button");
          btn.className =
            "pagination-page-btn" + (p === currentPage ? " active" : "");
          btn.textContent = p + 1;
          btn.disabled = p === currentPage;
          btn.addEventListener("click", () => {
            currentPage = p;
            renderPage();
            updateControls();
          });
          pn.appendChild(btn);
        }
      });
    }

    // Returns a sparse array of page indices with '…' gaps for large ranges.
    function pageRange(current, total) {
      if (total <= 7) return Array.from({ length: total }, (_, i) => i);
      const set = new Set(
        [0, total - 1, current - 1, current, current + 1].filter(
          (p) => p >= 0 && p < total,
        ),
      );
      const sorted = [...set].sort((a, b) => a - b);
      const result = [];
      sorted.forEach((p, i) => {
        if (i > 0 && p > sorted[i - 1] + 1) result.push("…");
        result.push(p);
      });
      return result;
    }
  }
})();
