(function () {
  var wrap = document.querySelector('[data-track-base-url]');
  var trackBaseUrl = wrap ? (wrap.getAttribute('data-track-base-url') || '') : '';

  var iconMarkup = {
    open: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M14 4h6v6"></path><path d="M10 14L20 4"></path><path d="M20 14v4a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h4"></path></svg>',
    copy: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><rect x="9" y="9" width="10" height="10"></rect><path d="M5 15V5h10"></path></svg>',
    download: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M12 3v12"></path><path d="M7 10l5 5 5-5"></path><path d="M5 21h14"></path></svg>',
    check: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M5 13l4 4L19 7"></path></svg>',
    spinner: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M12 3a9 9 0 1 0 9 9"></path></svg>'
  };

  function flashSuccess(button, fallbackKey) {
    button.classList.remove('is-loading');
    button.classList.add('is-success');
    button.innerHTML = iconMarkup.check;
    window.setTimeout(function () {
      button.classList.remove('is-success');
      button.innerHTML = iconMarkup[fallbackKey];
    }, 1200);
  }

  function trackMetric(eventName) {
    if (!trackBaseUrl) {
      return Promise.resolve(null);
    }

    var joiner = trackBaseUrl.indexOf('?') === -1 ? '?' : '&';
    return fetch(trackBaseUrl + joiner + 'event=' + encodeURIComponent(eventName), {
      method: 'POST',
      credentials: 'same-origin',
      headers: {
        'X-Requested-With': 'XMLHttpRequest'
      }
    }).then(function (response) {
      return response.json();
    }).catch(function () {
      return null;
    });
  }

  function updateMetricDisplay(id, value) {
    var target = document.getElementById(id);
    if (!target || typeof value !== 'number') {
      return;
    }
    target.textContent = String(value);
  }

  function inferDownloadName(url) {
    try {
      var parsed = new URL(url, window.location.href);
      var pathname = parsed.pathname || '';
      var fileName = pathname.split('/').pop() || 'image';
      return fileName || 'image';
    } catch (error) {
      return 'image';
    }
  }

  document.querySelectorAll('[data-open]').forEach(function (button) {
    button.addEventListener('click', function () {
      var target = button.getAttribute('data-open') || '';
      if (target) {
        window.open(target, '_blank', 'noopener');
      }
    });
  });

  document.querySelectorAll('[data-copy]').forEach(function (button) {
    button.addEventListener('click', async function () {
      try {
        await navigator.clipboard.writeText(button.getAttribute('data-copy') || '');
        flashSuccess(button, 'copy');
      } catch (error) {
        console.error(error);
      }
    });
  });

  document.querySelectorAll('[data-download]').forEach(function (button) {
    button.addEventListener('click', function () {
      var target = button.getAttribute('data-download') || '';
      if (!target || button.classList.contains('is-loading')) {
        return;
      }

      button.classList.remove('is-success');
      button.classList.add('is-loading');
      button.innerHTML = iconMarkup.spinner;

      window.setTimeout(function () {
        trackMetric('download').then(function (payload) {
          if (payload && payload.ok) {
            updateMetricDisplay('download-count', Number(payload.download_count || 0));
          }
        }).finally(function () {
          fetch(target, {
            credentials: 'same-origin'
          }).then(function (response) {
            if (!response.ok) {
              throw new Error('download_failed');
            }
            return response.blob();
          }).then(function (blob) {
            var objectUrl = window.URL.createObjectURL(blob);
            var link = document.createElement('a');
            link.href = objectUrl;
            link.download = inferDownloadName(target);
            link.style.display = 'none';
            document.body.appendChild(link);
            link.click();
            document.body.removeChild(link);
            window.setTimeout(function () {
              window.URL.revokeObjectURL(objectUrl);
            }, 1000);
            flashSuccess(button, 'download');
          }).catch(function (error) {
            console.error(error);
            button.classList.remove('is-loading');
            button.innerHTML = iconMarkup.download;
          });
        });
      }, 1000);
    });
  });

  if (window.ViewImage) {
    try {
      ViewImage.init('.preview img');
    } catch (error) {
      console.error(error);
    }
  }
}());
