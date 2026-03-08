(() => {
  // Shared state for widget picker — written by openPicker(), read by single confirm handler
  let widgetPickerSelection = { widget: null, size: null, filter: null };

  // ── Toast Notifications ──────────────────────────────────────
  function showToast(message, options = {}) {
    const {
      type = 'info', // 'error', 'warning', 'success', 'info'
      title = null,
      duration = 5000,
      closeable = true
    } = options;

    const container = document.getElementById('toast-container');
    if (!container) return;

    const toast = document.createElement('div');
    toast.className = `toast toast--${type}`;

    const icons = {
      error: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>',
      warning: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>',
      success: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 11-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>',
      info: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg>'
    };

    toast.innerHTML = `
      <div class="toast-icon toast-icon--${type}">${icons[type]}</div>
      <div class="toast-content">
        ${title ? `<div class="toast-title">${escapeHtml(title)}</div>` : ''}
        <div class="toast-message">${escapeHtml(message)}</div>
      </div>
      ${closeable ? '<button class="toast-close" aria-label="Dismiss">&times;</button>' : ''}
    `;

    const closeBtn = toast.querySelector('.toast-close');
    if (closeBtn) {
      closeBtn.addEventListener('click', () => removeToast(toast));
    }

    container.appendChild(toast);

    if (duration > 0) {
      setTimeout(() => removeToast(toast), duration);
    }

    return toast;
  }

  function removeToast(toast) {
    if (!toast || toast.classList.contains('toast--removing')) return;
    toast.classList.add('toast--removing');
    setTimeout(() => toast.remove(), 150);
  }

  function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }

  // Plugin metadata
  const PLUGIN_META = {
    github: {
      name: 'GitHub',
      icon: '<svg class="plugin-card-icon" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z"/></svg>',
      badgeIcon: '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z"/></svg>'
    }
  };

  // Helper to get the active filter value
  function getActiveFilter() {
    const activeLink = document.querySelector('.filters a.active');
    if (!activeLink) return '7d';
    const url = new URL(activeLink.href);
    return url.searchParams.get('filter') || '7d';
  }

  // Check for widget errors on page load and show toasts
  document.addEventListener('DOMContentLoaded', () => {
    const errorWidgets = document.querySelectorAll('.widget-error[data-error]');
    errorWidgets.forEach(widget => {
      const widgetId = widget.dataset.widgetId || 'unknown';
      const error = widget.dataset.error || 'Unknown error';
      showToast(error, {
        type: 'error',
        title: `Widget "${widgetId}" failed to load`,
        duration: 8000
      });
    });
  });

  const header = document.querySelector('.site-header');
  if (header) {
    let lastY = window.scrollY;
    window.addEventListener('scroll', () => {
      const y = window.scrollY;
      if (y > lastY && y > 80) {
        header.classList.add('site-header--hidden');
      } else {
        header.classList.remove('site-header--hidden');
      }
      lastY = y;
    }, { passive: true });
  }

  // ── Client-side navigation for filter links ────────────
  function initNav() {
    const main = document.querySelector('main.plugins');
    if (!main) return;

    main.addEventListener('click', (e) => {
      const link = e.target.closest('.filters a');
      if (!link) return;
      e.preventDefault();

      const url = link.href;
      navigateTo(url);
    });
  }

  let navController = null;

  function navigateTo(url) {
    const main = document.querySelector('main.plugins');
    if (!main) return;

    // Abort any in-flight navigation
    if (navController) navController.abort();
    navController = new AbortController();

    // Immediately update active filter
    const filters = main.querySelector('.filters');
    if (filters) {
      filters.querySelectorAll('a').forEach(a => {
        a.classList.toggle('active', a.href === url);
      });
    }

    main.classList.add('plugins--loading');

    fetch(url, { signal: navController.signal })
      .then(res => {
        if (!res.ok) throw new Error(res.statusText);
        return res.text();
      })
      .then(html => {
        const doc = new DOMParser().parseFromString(html, 'text/html');
        const newMain = doc.querySelector('main.plugins');
        if (!newMain) throw new Error('no main found');

        // Swap content and re-init components
        main.innerHTML = newMain.innerHTML;
        main.classList.remove('plugins--loading');
        main.querySelectorAll('.plugin, .widget').forEach(el => el.style.animation = 'none');
        initSortables();
        initWidgetGrid();

        // Update URL without reload
        history.pushState(null, '', url);

        // Update page title if changed
        const newTitle = doc.querySelector('title');
        if (newTitle) document.title = newTitle.textContent;
      })
      .catch(err => {
        if (err.name === 'AbortError') return;
        // On failure, fall back to normal navigation
        main.classList.remove('plugins--loading');
        window.location.href = url;
      });
  }

  // Handle back/forward
  window.addEventListener('popstate', () => {
    navigateTo(window.location.href);
  });

  function initSortables() {
    document.querySelectorAll('table[data-sortable]').forEach(initSortable);
  }

  // ── Tooltips for [data-tip] elements ────────────────────
  const tip = document.createElement('div');
  tip.className = 'tooltip';
  document.body.appendChild(tip);
  let tipTarget = null;

  document.addEventListener('pointerenter', (e) => {
    if (!e.target || typeof e.target.closest !== 'function') return;
    const el = e.target.closest('[data-tip]');
    if (!el || !el.dataset.tip) return;
    // For table cells, only show if content is truncated
    if (el.tagName === 'TD') {
      const measure = el.firstElementChild || el;
      if (measure.scrollWidth <= measure.clientWidth && el.scrollWidth <= el.clientWidth) return;
    }
    tipTarget = el;
    tip.textContent = el.dataset.tip;
    const rect = el.getBoundingClientRect();
    tip.style.left = rect.left + 'px';
    tip.style.top = (rect.top - tip.offsetHeight - 6) + 'px';
    tip.classList.add('tooltip--visible');
  }, true);

  document.addEventListener('pointerleave', (e) => {
    if (!e.target || typeof e.target.closest !== 'function') return;
    if (e.target.closest('[data-tip]') === tipTarget) {
      tip.classList.remove('tooltip--visible');
      tipTarget = null;
    }
  }, true);

  // ── Per-character hover effect for .title-text ────────
  document.querySelectorAll('.title-text').forEach(el => {
    const text = el.textContent;
    el.innerHTML = [...text].map(ch =>
      `<span class="title-char">${ch}</span>`
    ).join('');

    const h1 = el.closest('h1, .plugin-label') || el.parentElement;
    const chars = [...el.querySelectorAll('.title-char')];
    const basePad = 0.02;
    const peakPad = 0.14;
    const peakStroke = 0.8;
    const baseWeight = 500;
    const dimWeight = 300;

    h1.addEventListener('mousemove', (e) => {
      const h1Rect = h1.getBoundingClientRect();
      chars.forEach(ch => {
        const rect = ch.getBoundingClientRect();
        const center = rect.left + rect.width / 2;
        const dist = Math.abs(e.clientX - center) / h1Rect.width;
        const t = Math.max(0, 1 - dist * 4);
        ch.style.padding = `0 ${basePad + (peakPad - basePad) * t}em`;
        ch.style.webkitTextStroke = `${peakStroke * t}px var(--text)`;
        ch.style.fontWeight = dimWeight + (baseWeight - dimWeight) * t;
        ch.classList.toggle('title-char--near', t > 0.2);
      });
    });

    h1.addEventListener('mouseleave', () => {
      chars.forEach(ch => {
        ch.style.padding = `0 ${basePad}em`;
        ch.style.webkitTextStroke = '0px transparent';
        ch.style.fontWeight = baseWeight;
        ch.classList.remove('title-char--near');
      });
    });
  });

  initNav();
  initSortables();
  initWidgetGrid();
  initWidgetPickerConfirm();

  // ── Widget grid: edit mode, drag-and-drop, controls ────
  function initWidgetGrid() {
    const grid = document.getElementById('widget-grid');
    const editBtn = document.getElementById('edit-toggle');
    const saveBtn = document.getElementById('edit-save');
    const cancelBtn = document.getElementById('edit-cancel');
    const addCard = document.getElementById('widget-add-card');
    const emptyAddBtn = document.getElementById('empty-add-widget');

    if (emptyAddBtn) {
      emptyAddBtn.addEventListener('click', () => openPicker());
    }

    if (!grid || !editBtn) return;

    let editing = false;
    let sortable = null;
    let dirty = false;
    let widgetDefs = null;
    if (typeof Sortable !== 'undefined') {
      sortable = Sortable.create(grid, {
        animation: 200,
        ghostClass: 'widget--ghost',
        dragClass: 'widget--drag',
        draggable: '.widget:not(.widget-add)',
        handle: '.widget-drag-handle',
        filter: '.widget-remove, .widget-size-btn',
        preventOnFilter: false,
        disabled: true,
        forceFallback: true,
        fallbackOnBody: true,
        swapThreshold: 0.65,
        onEnd: () => { dirty = true; }
      });
    }

    function enterEditMode() {
      editing = true;
      grid.classList.add('widget-grid--editing');
      editBtn.style.display = 'none';
      if (saveBtn) saveBtn.style.display = '';
      if (cancelBtn) cancelBtn.style.display = '';
      if (addCard) addCard.style.display = '';
      if (sortable) sortable.option('disabled', false);
      dirty = false;
      populateSizePickers();
    }

    function exitEditMode() {
      editing = false;
      grid.classList.remove('widget-grid--editing');
      editBtn.style.display = '';
      if (saveBtn) saveBtn.style.display = 'none';
      if (cancelBtn) cancelBtn.style.display = 'none';
      if (addCard) addCard.style.display = 'none';
      if (sortable) sortable.option('disabled', true);
    }

    editBtn.addEventListener('click', enterEditMode);

    // Save — collect final state from DOM and bulk-save
    if (saveBtn) {
      saveBtn.addEventListener('click', () => {
        const widgets = [...grid.querySelectorAll('.widget[data-widget-id]')]
          .map(el => ({ id: el.dataset.widgetId, size_name: el.dataset.size }));
        fetch('/api/dashboard/widgets', {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ widgets })
        })
        .then(() => window.location.reload())
        .catch(err => console.error('Failed to save dashboard:', err));
      });
    }

    // Cancel — discard all changes by reloading
    if (cancelBtn) {
      cancelBtn.addEventListener('click', () => {
        window.location.reload();
      });
    }

    // Remove widget — controls are inside the widget, grid handles clicks
    grid.addEventListener('click', (e) => {
      const removeBtn = e.target.closest('.widget-remove');
      if (!removeBtn) return;
      const widgetEl = removeBtn.closest('.widget');
      if (widgetEl) {
        widgetEl.remove();
        dirty = true;
      }
    });

    // Size picker — controls are inside the widget, grid handles clicks
    grid.addEventListener('click', (e) => {
      const sizeBtn = e.target.closest('.widget-size-btn');
      if (!sizeBtn) return;
      const widgetEl = sizeBtn.closest('.widget');
      const id = widgetEl.dataset.widgetId;
      const sizeName = sizeBtn.dataset.sizeName;

      const def = widgetDefs && widgetDefs.find(w => w.id === id);
      const sizeOpt = def && def.sizes.find(s => s.name === sizeName);
      if (!sizeOpt) return;

      widgetEl.dataset.size = sizeName;
      widgetEl.style.gridColumn = `span ${sizeOpt.w}`;
      widgetEl.style.gridRow = `span ${sizeOpt.h}`;

      widgetEl.querySelectorAll('.widget-size-btn').forEach(b =>
        b.classList.toggle('active', b.dataset.sizeName === sizeName)
      );

      const filter = getActiveFilter();
      fetch(`/api/widgets/${encodeURIComponent(id)}/preview?size=${encodeURIComponent(sizeName)}&filter=${encodeURIComponent(filter)}`)
        .then(r => r.json())
        .then(data => {
          const fill = widgetEl.querySelector('.widget-fill');
          if (fill) {
            const temp = document.createElement('div');
            temp.innerHTML = data.html;
            const newFill = temp.querySelector('.widget-fill');
            if (newFill) fill.replaceWith(newFill);
          }
        })
        .catch(err => console.error('Failed to fetch widget preview:', err));

      dirty = true;
    });

    // Add widget card click
    if (addCard) {
      addCard.addEventListener('click', () => openPicker());
    }

    function populateSizePickers() {
      fetch('/api/widgets')
        .then(r => r.json())
        .then(widgets => {
          widgetDefs = widgets;
          const widgetMap = {};
          widgets.forEach(w => { widgetMap[w.id] = w; });

          grid.querySelectorAll('.widget-size-picker[data-widget-id]').forEach(picker => {
            const id = picker.dataset.widgetId;
            const def = widgetMap[id];
            if (!def) return;
            const currentSize = picker.closest('.widget').dataset.size;
            picker.innerHTML = def.sizes.map(s =>
              `<button class="widget-size-btn${s.name === currentSize ? ' active' : ''}" data-size-name="${s.name}">${s.name}</button>`
            ).join('');
          });
        })
        .catch(err => console.error('Failed to load widget definitions:', err));
    }
  }

  // ── Widget picker modal ─────────────────────────────────
  // Flow: Plugin list → Widget list → Preview with size picker
  function openPicker() {
    const overlay = document.getElementById('widget-picker-overlay');
    const listEl = document.getElementById('widget-picker-list');
    const previewEl = document.getElementById('widget-picker-preview');
    const previewArea = document.getElementById('widget-picker-preview-area');
    const previewTitle = document.getElementById('widget-picker-preview-title');
    const sizesEl = document.getElementById('widget-picker-sizes');
    const backBtn = document.getElementById('widget-picker-back');
    const pickerTitle = document.querySelector('.widget-picker-title');

    if (!overlay) return;

    widgetPickerSelection.widget = null;
    widgetPickerSelection.size = null;

    let allWidgets = [];
    let selectedPlugin = null;
    let selectedWidget = null;
    let selectedSize = null;

    overlay.classList.add('open');
    backBtn.style.display = 'none';
    showPluginList();

    // Get current filter
    const filter = getActiveFilter();

    // Fetch available widgets and show plugin groups
    fetch('/api/widgets')
      .then(r => r.json())
      .then(widgets => {
        allWidgets = widgets;
        renderPluginList();
      })
      .catch(err => console.error('Failed to load widgets:', err));

    function showPluginList() {
      listEl.style.display = '';
      previewEl.style.display = 'none';
      backBtn.style.display = 'none';
      if (pickerTitle) pickerTitle.textContent = 'Add Widget';
      selectedPlugin = null;
      selectedWidget = null;
      selectedSize = null;
    }

    function renderPluginList() {
      const pinned = new Set(
        [...document.querySelectorAll('.widget[data-widget-id]')].map(el => el.dataset.widgetId)
      );

      // Group by plugin_id
      const groups = {};
      allWidgets.forEach(w => {
        if (pinned.has(w.id)) return;
        if (!groups[w.plugin_id]) groups[w.plugin_id] = [];
        groups[w.plugin_id].push(w);
      });

      const pluginIds = Object.keys(groups);
      if (pluginIds.length === 0) {
        listEl.innerHTML = '<p style="color:var(--text-3);padding:1rem">All widgets are already pinned.</p>';
        return;
      }

      listEl.innerHTML = pluginIds.map(pid => {
        const count = groups[pid].length;
        const meta = PLUGIN_META[pid];
        return `
          <div class="widget-picker-item" data-plugin-id="${pid}">
            ${meta?.icon || ''}
            <div>
              <div class="widget-picker-item-name">${meta?.name || pid}</div>
              <div class="widget-picker-item-desc">${count} widget${count !== 1 ? 's' : ''} available</div>
            </div>
          </div>`;
      }).join('');

      listEl.querySelectorAll('.widget-picker-item[data-plugin-id]').forEach(item => {
        item.addEventListener('click', () => {
          selectedPlugin = item.dataset.pluginId;
          renderWidgetList(groups[selectedPlugin]);
        });
      });
    }

    function renderWidgetList(widgets) {
      backBtn.style.display = '';
      if (pickerTitle) pickerTitle.textContent = PLUGIN_META[selectedPlugin]?.name || selectedPlugin;
      listEl.innerHTML = widgets.map(w => `
        <div class="widget-picker-item" data-widget-id="${w.id}">
          <div>
            <div class="widget-picker-item-name">${w.name}</div>
            <div class="widget-picker-item-desc">${w.description}</div>
          </div>
          <div class="widget-picker-item-sizes">
            ${w.sizes.map(s => `<span class="widget-picker-size-dot" style="width:${s.w * 12}px;height:${s.h * 12}px"></span>`).join('')}
          </div>
        </div>
      `).join('');

      listEl.querySelectorAll('.widget-picker-item[data-widget-id]').forEach(item => {
        item.addEventListener('click', () => {
          const id = item.dataset.widgetId;
          selectedWidget = allWidgets.find(w => w.id === id);
          if (!selectedWidget) return;
          showPreview();
        });
      });
    }

    function showPreview() {
      listEl.style.display = 'none';
      previewEl.style.display = '';
      previewTitle.textContent = selectedWidget.name;

      selectedSize = selectedWidget.sizes[0].name;
      widgetPickerSelection.widget = selectedWidget;
      widgetPickerSelection.size = selectedSize;
      widgetPickerSelection.filter = filter;

      sizesEl.innerHTML = selectedWidget.sizes.map(s =>
        `<button class="widget-size-btn${s.name === selectedSize ? ' active' : ''}" data-size-name="${s.name}">${s.name} (${s.w}\u00d7${s.h})</button>`
      ).join('');

      loadPreview(selectedWidget.id, selectedSize, filter);

      sizesEl.querySelectorAll('.widget-size-btn').forEach(btn => {
        btn.addEventListener('click', () => {
          selectedSize = btn.dataset.sizeName;
          widgetPickerSelection.size = selectedSize;
          sizesEl.querySelectorAll('.widget-size-btn').forEach(b => b.classList.toggle('active', b === btn));
          loadPreview(selectedWidget.id, selectedSize, filter);
        });
      });
    }

    function loadPreview(id, size, f) {
      previewArea.innerHTML = '<p style="color:var(--text-3)">Loading...</p>';
      fetch(`/api/widgets/${encodeURIComponent(id)}/preview?size=${encodeURIComponent(size)}&filter=${encodeURIComponent(f)}`)
        .then(r => r.json())
        .then(data => {
          previewArea.innerHTML = data.html;
        })
        .catch(() => {
          previewArea.innerHTML = '<p style="color:var(--remove)">Failed to load preview.</p>';
        });
    }

    // Back — context-aware: preview → widget list → plugin list
    backBtn.addEventListener('click', () => {
      if (previewEl.style.display !== 'none') {
        // From preview → back to widget list
        listEl.style.display = '';
        previewEl.style.display = 'none';
        const pinned = new Set(
          [...document.querySelectorAll('.widget[data-widget-id]')].map(el => el.dataset.widgetId)
        );
        const pluginWidgets = allWidgets.filter(w => w.plugin_id === selectedPlugin && !pinned.has(w.id));
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
    const overlay = document.getElementById('widget-picker-overlay');
    if (overlay) overlay.classList.remove('open');
  }

  function initWidgetPickerConfirm() {
    const overlay = document.getElementById('widget-picker-overlay');
    const closeBtn = document.getElementById('widget-picker-close');
    const confirmBtn = document.getElementById('widget-picker-confirm');
    if (!overlay) return;

    if (closeBtn) closeBtn.addEventListener('click', closeWidgetPicker);
    overlay.addEventListener('click', (e) => {
      if (e.target === overlay) closeWidgetPicker();
    });

    if (!confirmBtn) return;
    confirmBtn.addEventListener('click', () => {
      const selectedWidget = widgetPickerSelection.widget;
      const selectedSize = widgetPickerSelection.size;
      const filter = widgetPickerSelection.filter || getActiveFilter();
      if (!selectedWidget || !selectedSize) return;
      const sizeOpt = selectedWidget.sizes.find(s => s.name === selectedSize);
      if (!sizeOpt) return;

      const grid = document.getElementById('widget-grid');
      if (!grid) {
        fetch('/api/dashboard/widgets', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ id: selectedWidget.id, size_name: selectedSize })
        })
          .then(() => window.location.reload())
          .catch(err => console.error('Failed to pin widget:', err));
        return;
      }

      fetch(`/api/widgets/${encodeURIComponent(selectedWidget.id)}/preview?size=${encodeURIComponent(selectedSize)}&filter=${encodeURIComponent(filter)}`)
        .then(r => r.json())
        .then(data => {
          const grid = document.getElementById('widget-grid');
          const addCard = document.getElementById('widget-add-card');
          if (!grid) return;

          const pid = selectedWidget.plugin_id;
          const meta = PLUGIN_META[pid];

          const div = document.createElement('div');
          div.className = 'widget';
          div.dataset.widgetId = selectedWidget.id;
          div.dataset.size = selectedSize;
          div.style.gridColumn = `span ${sizeOpt.w}`;
          div.style.gridRow = `span ${sizeOpt.h}`;
          div.innerHTML = `
            <div class="widget-controls">
              <span class="widget-drag-handle" title="Drag to reorder" aria-hidden="true">⋮⋮</span>
              <button class="widget-remove" data-widget-id="${selectedWidget.id}" title="Remove widget">&times;</button>
            </div>
            <div class="widget-size-picker" data-widget-id="${selectedWidget.id}"></div>
            ${meta?.badgeIcon ? `<div class="widget-badge" title="${meta.name || pid}">${meta.badgeIcon}</div>` : ''}
            ${data.html}`;

          if (addCard) grid.insertBefore(div, addCard);
          else grid.appendChild(div);

          const picker = div.querySelector('.widget-size-picker');
          if (picker) {
            picker.innerHTML = selectedWidget.sizes.map(s =>
              `<button class="widget-size-btn${s.name === selectedSize ? ' active' : ''}" data-size-name="${s.name}">${s.name}</button>`
            ).join('');
          }

          closeWidgetPicker();
        })
        .catch(err => console.error('Failed to fetch widget preview:', err));
    });
  }

  // ── Sortable tables with pagination ────────────────────
  function initSortable(table) {
    const PAGE_SIZE = 10;
    const tbody = table.tBodies[0];
    let sortedRows = [...tbody.rows];
    let currentPage = 0;
    let currentCol = null;
    let ascending = true;

    const defaultTh = table.querySelector('th[data-sort-default]');
    if (defaultTh) sort(defaultTh, false);
    else renderPage();

    if (sortedRows.length > PAGE_SIZE) {
      // wrap in div to lock height
      const wrapper = document.createElement('div');
      table.replaceWith(wrapper);
      wrapper.appendChild(table);

      injectControls(wrapper);

      document.fonts.ready.then(() => {
        wrapper.style.minHeight = wrapper.offsetHeight + 'px';
      });
    }

    table.querySelectorAll('th[data-col]').forEach(th => {
      th.addEventListener('click', () => {
        const isActive = th === currentCol;
        sort(th, isActive ? !ascending : true);
      });
    });

    function sort(th, asc) {
      const colIndex = [...th.parentElement.children].indexOf(th);
      sortedRows.sort((a, b) => {
        const av = a.cells[colIndex]?.dataset.val ?? '';
        const bv = b.cells[colIndex]?.dataset.val ?? '';
        const an = Number(av), bn = Number(bv);
        const cmp = (!isNaN(an) && !isNaN(bn)) ? an - bn : av.localeCompare(bv);
        return asc ? cmp : -cmp;
      });
      sortedRows.forEach(r => tbody.appendChild(r));
      table.querySelectorAll('th[data-col]').forEach(h => h.removeAttribute('aria-sort'));
      th.setAttribute('aria-sort', asc ? 'ascending' : 'descending');
      currentCol = th;
      ascending = asc;
      currentPage = 0;
      renderPage();
      updateControls();
    }

    function renderPage() {
      const start = currentPage * PAGE_SIZE;
      const end = start + PAGE_SIZE;
      sortedRows.forEach((r, i) => { r.hidden = i < start || i >= end; });
    }

    function injectControls(anchor) {
      const container = document.createElement('div');
      container.className = 'table-pagination';

      const prev = document.createElement('button');
      prev.className = 'pagination-btn';
      prev.textContent = '←';
      prev.addEventListener('click', () => {
        if (currentPage > 0) { currentPage--; renderPage(); updateControls(); }
      });

      const pageNums = document.createElement('div');
      pageNums.className = 'pagination-pages';

      const next = document.createElement('button');
      next.className = 'pagination-btn';
      next.textContent = '→';
      next.addEventListener('click', () => {
        if (currentPage < Math.ceil(sortedRows.length / PAGE_SIZE) - 1) {
          currentPage++; renderPage(); updateControls();
        }
      });

      const info = document.createElement('span');
      info.className = 'pagination-info';

      container.appendChild(prev);
      container.appendChild(pageNums);
      container.appendChild(next);
      container.appendChild(info);
      anchor.insertAdjacentElement('afterend', container);

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
      pn.innerHTML = '';
      pageRange(currentPage, totalPages).forEach(p => {
        if (p === '…') {
          const sep = document.createElement('span');
          sep.className = 'pagination-ellipsis';
          sep.textContent = '…';
          pn.appendChild(sep);
        } else {
          const btn = document.createElement('button');
          btn.className = 'pagination-page-btn' + (p === currentPage ? ' active' : '');
          btn.textContent = p + 1;
          btn.disabled = p === currentPage;
          btn.addEventListener('click', () => {
            currentPage = p; renderPage(); updateControls();
          });
          pn.appendChild(btn);
        }
      });
    }

    // Returns a sparse array of page indices with '…' gaps for large ranges.
    function pageRange(current, total) {
      if (total <= 7) return Array.from({ length: total }, (_, i) => i);
      const set = new Set(
        [0, total - 1, current - 1, current, current + 1].filter(p => p >= 0 && p < total)
      );
      const sorted = [...set].sort((a, b) => a - b);
      const result = [];
      sorted.forEach((p, i) => {
        if (i > 0 && p > sorted[i - 1] + 1) result.push('…');
        result.push(p);
      });
      return result;
    }
  }
})();
