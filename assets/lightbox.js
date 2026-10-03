import PhotoSwipeLightbox from './photoswipe-lightbox.esm.min.js';

// Photo anchors carry their lightbox data as attributes:
//   href, data-pswp-width/-height/-srcset  read by PhotoSwipe itself
//   data-name, data-date, data-dsrc, data-orig, data-album, data-album-name

export function formatDate(iso, dsrc) {
  if (!iso) return '';
  const d = new Date(iso);
  if (dsrc === 'year') return String(d.getFullYear());
  const opts = { year: 'numeric', month: 'long', day: 'numeric' };
  if (dsrc === 'exif' || (dsrc === 'name' && !iso.endsWith('T00:00:00'))) {
    opts.hour = '2-digit';
    opts.minute = '2-digit';
  }
  return d.toLocaleString(undefined, opts);
}

export function createLightbox(options = {}) {
  const lightbox = new PhotoSwipeLightbox({
    pswpModule: () => import('./photoswipe.esm.min.js'),
    bgOpacity: 0.95,
    loop: false,
    showHideAnimationType: 'zoom',
    ...options,
  });

  lightbox.addFilter('itemData', (data) => {
    const el = data.element;
    if (el && el.dataset) {
      data.name = el.dataset.name;
      data.date = el.dataset.date;
      data.dsrc = el.dataset.dsrc;
      data.orig = el.dataset.orig;
      data.album = el.dataset.album;
      data.albumName = el.dataset.albumName;
    }
    return data;
  });

  lightbox.on('uiRegister', () => {
    const pswp = lightbox.pswp;
    pswp.ui.registerElement({
      name: 'download-button',
      order: 8,
      isButton: true,
      tagName: 'a',
      title: 'Download original',
      html: {
        isCustomSVG: true,
        inner: '<path d="M20.5 14.3 17.1 18V10h-2.2v7.9l-3.4-3.6L10 16l6 6.1 6-6.1ZM23 23H9v2h14Z" id="pswp__icn-download"/>',
        outlineID: 'pswp__icn-download',
      },
      onInit: (el, pswp) => {
        el.setAttribute('download', '');
        el.setAttribute('target', '_blank');
        el.setAttribute('rel', 'noopener');
        pswp.on('change', () => {
          el.href = pswp.currSlide.data.orig || pswp.currSlide.data.src;
        });
      },
    });
    pswp.ui.registerElement({
      name: 'gosgal-caption',
      order: 9,
      isButton: false,
      appendTo: 'root',
      onInit: (el, pswp) => {
        pswp.on('change', () => {
          const d = pswp.currSlide.data;
          el.replaceChildren();
          const line = document.createElement('div');
          line.textContent = formatDate(d.date, d.dsrc);
          el.append(line);
          const sub = document.createElement('div');
          sub.className = 'sub';
          if (d.album) {
            const a = document.createElement('a');
            a.href = d.album;
            a.textContent = d.albumName;
            sub.append(a, ' · ');
          }
          sub.append(d.name || '');
          el.append(sub);
        });
      },
    });
  });

  return lightbox;
}
