    (function () {
      var primarySiteBaseUrl = 'https://img.et';
      var widthInput = document.getElementById('tester-width');
      var heightInput = document.getElementById('tester-height');
      var typeInput = document.getElementById('tester-type');
      var formatInput = document.getElementById('tester-format');
      var modeInput = document.getElementById('tester-mode');
      var rField = document.getElementById('tester-r-field');
      var rInput = document.getElementById('tester-r');
      var randomizeRButton = document.getElementById('tester-randomize-r');
      var slotInput = document.getElementById('tester-slot');
      var urlOutput = document.getElementById('tester-url');
      var openLink = document.getElementById('tester-open');
      var copyButton = document.getElementById('tester-copy');

      if (!widthInput || !heightInput || !typeInput || !formatInput || !modeInput || !rField || !rInput || !slotInput || !urlOutput || !openLink || !copyButton || !randomizeRButton) {
        return;
      }

      function generateRandomRValue() {
        var length = Math.floor(Math.random() * 5) + 2;
        var min = Math.pow(10, length - 1);
        var max = Math.pow(10, length) - 1;
        return String(Math.floor(Math.random() * (max - min + 1)) + min);
      }

      function buildTesterUrl() {
        var width = String(parseInt(widthInput.value, 10) || 1920);
        var height = String(parseInt(heightInput.value, 10) || 1080);
        var params = new URLSearchParams();
        var isFixedMode = modeInput.value === 'fixed';

        if (typeInput.value) {
          params.set('type', typeInput.value);
        }

        if (formatInput.value) {
          params.set('format', formatInput.value);
        }

        if (isFixedMode && rInput.value.trim()) {
          params.set('r', rInput.value.trim());
        }

        if (slotInput.value.trim()) {
          params.set('s', slotInput.value.trim());
        }

        var url = primarySiteBaseUrl + '/' + width + '/' + height;
        var query = params.toString();
        if (query) {
          url += '?' + query;
        }

        urlOutput.textContent = url;
        openLink.href = url;
        rField.hidden = !isFixedMode;
        rInput.disabled = !isFixedMode;
      }

      [widthInput, heightInput, typeInput, formatInput, modeInput, rInput, slotInput].forEach(function (element) {
        element.addEventListener('input', buildTesterUrl);
        element.addEventListener('change', buildTesterUrl);
      });

      copyButton.addEventListener('click', async function () {
        try {
          await navigator.clipboard.writeText(urlOutput.textContent || '');
          copyButton.textContent = '已复制';
          window.setTimeout(function () {
            copyButton.textContent = '复制地址';
          }, 1400);
        } catch (error) {
          console.error(error);
        }
      });

      randomizeRButton.addEventListener('click', function () {
        rInput.value = generateRandomRValue();
        buildTesterUrl();
      });

      buildTesterUrl();
    }());

    document.querySelectorAll('.copy-btn').forEach(function (button) {
      button.addEventListener('click', async function () {
        var targetId = button.getAttribute('data-copy-target');
        var target = targetId ? document.getElementById(targetId) : null;
        if (!target) {
          return;
        }

        var text = (target.textContent || '').trim();
        try {
          await navigator.clipboard.writeText(text);
          button.classList.add('copied');
          window.setTimeout(function () {
            button.classList.remove('copied');
          }, 1400);
        } catch (error) {
          console.error(error);
        }
      });
    });

    (function () {
      var cards = Array.prototype.slice.call(document.querySelectorAll('.sample-card'));
      if (cards.length === 0) {
        return;
      }

      function bindImage(card) {
        var image = card.querySelector('img.lazyload, img[data-src], img');
        if (!image) {
          return;
        }

        function clearLoading() {
          card.classList.remove('is-loading');
          card.classList.add('is-loaded');
        }

        image.addEventListener('load', clearLoading, { once: true });
        image.addEventListener('error', function () {
          card.classList.remove('is-loading');
        }, { once: true });

        if (image.complete && image.currentSrc) {
          clearLoading();
        }
      }

      cards.forEach(bindImage);

      if (!('IntersectionObserver' in window)) {
        cards.forEach(function (card) {
          if (!card.classList.contains('is-loaded')) {
            card.classList.add('is-loading');
          }
        });
        return;
      }

      var observer = new IntersectionObserver(function (entries) {
        entries.forEach(function (entry) {
          if (!entry.isIntersecting) {
            return;
          }

          var card = entry.target;
          if (!card.classList.contains('is-loaded')) {
            card.classList.add('is-loading');
          }
          observer.unobserve(card);
        });
      }, {
        rootMargin: '120px 0px'
      });

      cards.forEach(function (card) {
        observer.observe(card);
      });
    }());

    (function () {
      var modal = document.getElementById('site-info-modal');
      if (!modal) {
        return;
      }

      var triggerButtons = Array.prototype.slice.call(document.querySelectorAll('[data-modal-tab]'));
      var closeButtons = Array.prototype.slice.call(modal.querySelectorAll('[data-modal-close]'));
      var tabs = Array.prototype.slice.call(modal.querySelectorAll('.site-modal-tab'));
      var panels = Array.prototype.slice.call(modal.querySelectorAll('.site-modal-panel'));

      function setActiveTab(tabName) {
        tabs.forEach(function (tab) {
          var active = tab.getAttribute('data-tab') === tabName;
          tab.classList.toggle('is-active', active);
          tab.setAttribute('aria-selected', active ? 'true' : 'false');
        });

        panels.forEach(function (panel) {
          var active = panel.getAttribute('data-panel') === tabName;
          panel.classList.toggle('is-active', active);
        });
      }

      function openModal(tabName) {
        setActiveTab(tabName || 'usage');
        modal.hidden = false;
      }

      function closeModal() {
        modal.hidden = true;
      }

      triggerButtons.forEach(function (button) {
        button.addEventListener('click', function () {
          openModal(button.getAttribute('data-modal-tab') || 'usage');
        });
      });

      tabs.forEach(function (tab) {
        tab.addEventListener('click', function () {
          setActiveTab(tab.getAttribute('data-tab') || 'usage');
        });
      });

      closeButtons.forEach(function (button) {
        button.addEventListener('click', closeModal);
      });

      document.addEventListener('keydown', function (event) {
        if (event.key === 'Escape' && !modal.hidden) {
          closeModal();
        }
      });
    }());
