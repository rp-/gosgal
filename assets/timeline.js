import { createLightbox } from './lightbox.js';

const dataURL = document.body.dataset.timeline;
const main = document.getElementById('timeline');
const yearsEl = document.getElementById('years');
const currentEl = document.getElementById('current-month');
const bar = document.querySelector('.bar');

const index = await fetch(dataURL + 'index.json').then((r) => r.json());
const albums = index.albums;

const monthFmt = new Intl.DateTimeFormat(undefined, { month: 'long', year: 'numeric' });
const monthLabel = (m) => monthFmt.format(new Date(+m.slice(0, 4), +m.slice(5, 7) - 1, 1));

// one section per month, newest first; photos are fetched when a section gets close to the viewport
const sections = index.months.map((mo, i) => {
  const el = document.createElement('section');
  el.className = 'month';
  el.id = mo.m;
  const h = document.createElement('h2');
  h.textContent = monthLabel(mo.m);
  const grid = document.createElement('div');
  grid.className = 'grid';
  el.append(h, grid);
  main.append(el);
  const s = { i, m: mo.m, n: mo.n, el, grid, loaded: false, loading: null, anchors: [] };
  el.gosgalSection = s;
  return s;
});

function estimateHeights() {
  const style = getComputedStyle(document.documentElement);
  const row = parseFloat(style.getPropertyValue('--row')) || 220;
  const gap = parseFloat(style.getPropertyValue('--gap')) || 4;
  const perRow = Math.max(1, Math.floor(main.clientWidth / (row * 1.4 + gap)));
  for (const s of sections) {
    if (!s.loaded) s.grid.style.minHeight = Math.ceil(s.n / perRow) * (row + gap) + 'px';
  }
}
estimateHeights();

function photoEl(p) {
  const al = albums[p.al];
  const a = document.createElement('a');
  a.href = al.u + p.s;
  a.style.setProperty('--a', p.a);
  const ds = a.dataset;
  ds.pswpWidth = p.w;
  ds.pswpHeight = p.h;
  if (p.ss) ds.pswpSrcset = p.ss;
  ds.orig = al.u + p.o;
  ds.name = p.n;
  ds.date = p.d;
  ds.dsrc = p.ds;
  ds.album = al.u;
  ds.albumName = al.n;
  const img = document.createElement('img');
  img.loading = 'lazy';
  img.alt = p.n;
  img.src = al.u + p.t;
  a.append(img);
  return a;
}

function load(s) {
  if (!s.loading) {
    s.loading = fetch(dataURL + s.m + '.json')
      .then((r) => r.json())
      .then((items) => {
        const frag = document.createDocumentFragment();
        for (const p of items) frag.append(photoEl(p));
        // keep the visible content in place if a section above the viewport changes its height
        const above = s.el.getBoundingClientRect().bottom <= bar.offsetHeight;
        const oldHeight = s.el.offsetHeight;
        s.grid.append(frag);
        s.grid.style.minHeight = '';
        s.anchors = [...s.grid.children];
        s.loaded = true;
        if (above) window.scrollBy(0, s.el.offsetHeight - oldHeight);
        observer.unobserve(s.el);
      })
      .catch((err) => {
        s.loading = null;
        throw err;
      });
  }
  return s.loading;
}

const observer = new IntersectionObserver(
  (entries) => {
    for (const e of entries) if (e.isIntersecting) load(e.target.gosgalSection);
  },
  { rootMargin: '1500px 0px' },
);
for (const s of sections) observer.observe(s.el);

// year jump bar
const yearLinks = new Map();
for (const s of sections) {
  const y = s.m.slice(0, 4);
  if (yearLinks.has(y)) continue;
  const a = document.createElement('a');
  a.href = '#' + s.m;
  a.textContent = y;
  a.addEventListener('click', (e) => {
    e.preventDefault();
    jumpTo(s);
  });
  yearsEl.append(a);
  yearLinks.set(y, a);
}

function jumpTo(s) {
  window.scrollTo({ top: s.el.offsetTop - bar.offsetHeight });
  history.replaceState(null, '', '#' + s.m);
}

// current month in the top bar
let current = null;
function updateCurrent() {
  const y = bar.offsetHeight + 1;
  let lo = 0;
  let hi = sections.length - 1;
  while (lo < hi) {
    const mid = (lo + hi) >> 1;
    if (sections[mid].el.getBoundingClientRect().bottom > y) hi = mid;
    else lo = mid + 1;
  }
  const s = sections[lo];
  if (!s || s === current) return;
  if (current) yearLinks.get(current.m.slice(0, 4))?.classList.remove('active');
  yearLinks.get(s.m.slice(0, 4))?.classList.add('active');
  currentEl.textContent = monthLabel(s.m);
  current = s;
  history.replaceState(null, '', '#' + s.m);
}
let ticking = false;
window.addEventListener(
  'scroll',
  () => {
    if (ticking) return;
    ticking = true;
    requestAnimationFrame(() => {
      ticking = false;
      updateCurrent();
    });
  },
  { passive: true },
);

let resizeTimer;
window.addEventListener('resize', () => {
  clearTimeout(resizeTimer);
  resizeTimer = setTimeout(estimateHeights, 200);
});

// lightbox over all loaded months around the clicked photo, extended forward while swiping
const lightbox = createLightbox();
let run = null;
let extending = false;

main.addEventListener('click', async (e) => {
  const a = e.target.closest('.grid > a');
  if (!a || e.button !== 0 || e.ctrlKey || e.metaKey || e.shiftKey || e.altKey) return;
  e.preventDefault();
  const s = a.closest('section').gosgalSection;
  await Promise.allSettled([sections[s.i - 1], sections[s.i + 1]].filter(Boolean).map(load));
  let first = s.i;
  let last = s.i;
  while (first > 0 && sections[first - 1].loaded) first--;
  while (last < sections.length - 1 && sections[last + 1].loaded) last++;
  run = { last };
  const items = sections.slice(first, last + 1).flatMap((x) => x.anchors);
  lightbox.loadAndOpen(items.indexOf(a), items);
});

lightbox.on('change', () => {
  const pswp = lightbox.pswp;
  const items = pswp.options.dataSource;
  if (extending || !run || run.last >= sections.length - 1 || pswp.currIndex < items.length - 5) return;
  extending = true;
  const next = sections[run.last + 1];
  load(next)
    .then(() => {
      if (lightbox.pswp !== pswp) return;
      const wasLast = pswp.currIndex === items.length - 1;
      items.push(...next.anchors);
      run.last++;
      if (wasLast) pswp.refreshSlideContent(pswp.currIndex + 1);
    })
    .finally(() => {
      extending = false;
    });
});

lightbox.on('close', () => {
  lightbox.pswp.currSlide?.data.element?.scrollIntoView({ block: 'nearest' });
});

lightbox.init();

const target = sections.find((s) => '#' + s.m === location.hash);
if (target) jumpTo(target);
updateCurrent();
