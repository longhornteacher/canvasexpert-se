(function () {
  'use strict';
  const button = document.querySelector('[data-appearance-toggle]');
  if (!button) return;
  const root = document.documentElement;
  const system = window.matchMedia('(prefers-color-scheme: dark)');
  const label = button.querySelector('[data-appearance-label]');
  const status = button.parentElement.querySelector('[role="status"]');
  function effectiveMode() {
    return root.dataset.appearance || (system.matches ? 'dark' : 'light');
  }
  function update() {
    const mode = effectiveMode();
    button.setAttribute('aria-checked', String(mode === 'dark'));
    label.textContent = mode === 'dark' ? 'Dark' : 'Light';
  }
  system.addEventListener('change', update);
  update();
  button.addEventListener('click', async function () {
    if (button.disabled) return;
    const mode = effectiveMode() === 'dark' ? 'light' : 'dark';
    root.dataset.appearance = mode;
    update();
    button.disabled = true;
    status.textContent = 'Saving…';
    try {
      const body = new URLSearchParams();
      body.set('request_secret', button.dataset.requestSecret);
      body.set('mode', mode);
      const response = await fetch('/_cse/appearance', {
        method: 'POST', credentials: 'same-origin', body
      });
      if (!response.ok) throw new Error('save failed');
      const saved = await response.json();
      if (saved.mode !== mode) throw new Error('unexpected response');
      status.textContent = 'Saved';
    } catch (_) {
      status.textContent = 'This appearance could not be saved. Your work is still here.';
    } finally {
      button.disabled = false;
    }
  });
})();
