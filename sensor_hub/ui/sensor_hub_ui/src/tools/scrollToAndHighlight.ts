export function scrollToAndHighlight(elementId: string) {
  const el = document.getElementById(elementId);
  if (!el) return;

  el.scrollIntoView({ behavior: 'smooth', block: 'center' });

  el.style.transition = 'box-shadow 0.3s ease';
  el.style.boxShadow = '0 0 0 3px var(--mui-palette-primary-main)';

  setTimeout(() => {
    el.style.boxShadow = '';
    setTimeout(() => { el.style.transition = ''; }, 300);
  }, 1500);
}
