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

  document.querySelectorAll('table[data-sortable]').forEach(initSortable);

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
