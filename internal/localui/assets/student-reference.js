(() => {
  'use strict';
  const button = document.getElementById('print-student-reference');
  if (button) button.addEventListener('click', () => window.print());
})();
