(function () {
  var filters = document.getElementById('filters');
  if (!filters) return;

  window.addEventListener('scroll', function () {
    if (window.scrollY > 60) {
      filters.classList.add('hidden');
    } else {
      filters.classList.remove('hidden');
    }
  }, { passive: true });
}());