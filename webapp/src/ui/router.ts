import SimpleRouter from '@catalystcommunity/simple-router';
import { cell, cleanup, sample } from './state';
import { defineView, el, live, type Attributes, type Content, type View } from './dom';

const [url, setURL] = cell(new URL(window.location.href));
let activeRouter: SimpleRouter | undefined;
export function navigation() {
  return (path: string, options: { replace?: boolean } = {}) => {
    if (options.replace) {
      window.history.replaceState({}, '', path);
      setURL(new URL(window.location.href));
      activeRouter?.route('replace');
    } else if (activeRouter) activeRouter.changePage(path);
    else { window.history.pushState({}, '', path); setURL(new URL(window.location.href)); }
  };
}
export function locationState() { return { get pathname() { return url().pathname; }, get search() { return url().search; }, get hash() { return url().hash; } }; }
export function searchState<T extends Record<string, unknown> = Record<string, string | undefined>>(): [T, (patch: Partial<Record<keyof T, string | undefined>>, options?: {replace?: boolean}) => void] {
  const params = new Proxy({} as T, { get: (_, key) => url().searchParams.get(String(key)) ?? undefined });
  return [params, (patch, options) => {
    const next = new URL(url());
    for (const [key, value] of Object.entries(patch)) if (value === undefined) next.searchParams.delete(key); else next.searchParams.set(key, String(value));
    navigation()(next.pathname + next.search + next.hash, options);
  }];
}
export function routeParams<T extends Record<string, string>>(): T {
  return new Proxy({} as T, { get: (_, key) => key === 'slug' ? decodeURIComponent(url().pathname.split('/')[2] ?? '') : undefined });
}
export function Link(props: Attributes<HTMLAnchorElement> & { href: string }): HTMLAnchorElement {
  return el('a', props);
}
export interface PageRoute { path: string; page: View; }
export function outlet(routes: PageRoute[], fallback: View): Content {
  const router = new SimpleRouter(); activeRouter = router;
  const [selected, select] = cell<View>(fallback);
  router.eventSender = () => setURL(new URL(window.location.href));
  for (const route of routes) router.registerRoute(route.path, () => select(() => route.page));
  router.registerRoute('/notfound', () => select(() => fallback), [], true);
  const click = (event: MouseEvent) => {
    if (event.defaultPrevented || event.button || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return;
    const anchor = event.composedPath().find(node => node instanceof HTMLAnchorElement) as HTMLAnchorElement | undefined;
    if (!anchor || !anchor.hasAttribute('href') || anchor.origin !== location.origin || anchor.target || anchor.hasAttribute('download') || anchor.relList.contains('external') || anchor.pathname.startsWith('/api/')) return;
    if (anchor.pathname === location.pathname && anchor.search === location.search && anchor.hash) return;
    event.preventDefault(); navigation()(anchor.pathname + anchor.search + anchor.hash);
  };
  document.addEventListener('click', click);
  cleanup(() => { document.removeEventListener('click', click); router.cleanup(); if (activeRouter === router) activeRouter = undefined; });
  router.route();
  return live(() => { const page = selected(); return sample(() => defineView('longhouse-page', page)({})); });
}
// Test navigation uses the same URL state and browser event path as the app.
export function resetLocation(path = '/') { window.history.replaceState({}, '', path); setURL(new URL(window.location.href)); }
