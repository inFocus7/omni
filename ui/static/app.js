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
    const headers = table.querySelectorAll('th[data-col]');
    let currentCol = null;
    let ascending = true;

    // Apply default sort if one is marked
    const defaultTh = table.querySelector('th[data-sort-default]');
    if (defaultTh) sort(defaultTh, table, false);

    headers.forEach(th => {
      th.addEventListener('click', () => {
        const isActive = th === currentCol;
        sort(th, table, isActive ? !ascending : true);
      });
    });

    function sort(th, table, asc) {
      const col = th.dataset.col;
      const colIndex = [...th.parentElement.children].indexOf(th);

      const rows = [...table.tBodies[0].rows];
      rows.sort((a, b) => {
        const av = a.cells[colIndex]?.dataset.val ?? '';
        const bv = b.cells[colIndex]?.dataset.val ?? '';
        const an = Number(av), bn = Number(bv);
        const cmp = (!isNaN(an) && !isNaN(bn))
          ? an - bn
          : av.localeCompare(bv);
        return asc ? cmp : -cmp;
      });

      rows.forEach(r => table.tBodies[0].appendChild(r));

      // Update aria-sort on all headers
      table.querySelectorAll('th[data-col]').forEach(h => {
        h.removeAttribute('aria-sort');
      });
      th.setAttribute('aria-sort', asc ? 'ascending' : 'descending');

      currentCol = th;
      ascending = asc;
    }
  }
})();
