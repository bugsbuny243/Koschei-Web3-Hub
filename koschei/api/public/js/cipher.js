(() => {
  'use strict';

  const CIPHER_CHARS = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789@#$%&*';
  const DEFAULT_DURATION = 1200;
  const FRAME_INTERVAL = 1000 / 60;

  const toMs = (value, fallback) => {
    const parsed = Number(value);
    return Number.isFinite(parsed) && parsed >= 0 ? parsed : fallback;
  };

  const randomChar = () => CIPHER_CHARS[(Math.random() * CIPHER_CHARS.length) | 0];

  const decrypt = (el) => {
    const finalText = String(el.dataset.cipher || '');
    const duration = toMs(el.dataset.duration, DEFAULT_DURATION);
    const delay = toMs(el.dataset.delay, 0);
    const length = finalText.length;
    let startTime = 0;
    let lastFrame = 0;

    el.textContent = ''.padEnd(length, '█');
    el.setAttribute('aria-label', finalText);

    const render = (now) => {
      if (!startTime) startTime = now;

      if (now - lastFrame < FRAME_INTERVAL) {
        requestAnimationFrame(render);
        return;
      }

      lastFrame = now;
      const elapsed = Math.min(now - startTime, duration);
      const progress = duration === 0 ? 1 : elapsed / duration;
      const locked = Math.floor(progress * length);
      let output = '';

      for (let i = 0; i < length; i += 1) {
        const target = finalText[i];
        if (i < locked || target === ' ') {
          output += target;
        } else {
          output += randomChar();
        }
      }

      el.textContent = output;

      if (progress < 1) {
        requestAnimationFrame(render);
      } else {
        el.textContent = finalText;
        el.dataset.cipherComplete = 'true';
      }
    };

    window.setTimeout(() => requestAnimationFrame(render), delay);
  };

  document.addEventListener('DOMContentLoaded', () => {
    document.querySelectorAll('[data-cipher]').forEach(decrypt);
  });
})();
