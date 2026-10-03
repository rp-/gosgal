import { createLightbox } from './lightbox.js';

if (document.getElementById('photos')) {
  const lightbox = createLightbox({ gallery: '#photos', children: 'a' });
  lightbox.init();
}
