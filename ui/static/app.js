(() => {
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
        main.querySelectorAll('.plugin').forEach(el => el.style.animation = 'none');
        initSortables();

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
