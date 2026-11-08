import { BaseComponent } from '@catalystcommunity/base-components';
import { cell, cleanup, currentLifetime, flush, inLifetime, observe, sample, scope, type Lifetime, type Read } from './state';

export type Content = Node | string | number | boolean | null | undefined | Content[];
export type View<P = {}> = (props: P) => Content;
export type ParentView<P = {}> = View<P & { children?: Content }>;
type DOMEvent<T, E extends Event> = E & { currentTarget: T; target: Element };
export interface Attributes<T extends Element = HTMLElement> {
  children?: Content;
  ref?: (element: T) => void;
  onClick?: (event: DOMEvent<T, MouseEvent>) => unknown;
  onDblClick?: (event: DOMEvent<T, MouseEvent>) => unknown;
  onInput?: (event: DOMEvent<T, InputEvent>) => unknown;
  onChange?: (event: DOMEvent<T, Event>) => unknown;
  onSubmit?: (event: DOMEvent<T, SubmitEvent>) => unknown;
  onKeyDown?: (event: DOMEvent<T, KeyboardEvent>) => unknown;
  onKeyUp?: (event: DOMEvent<T, KeyboardEvent>) => unknown;
  onMouseDown?: (event: DOMEvent<T, MouseEvent>) => unknown;
  onMouseUp?: (event: DOMEvent<T, MouseEvent>) => unknown;
  onMouseEnter?: (event: DOMEvent<T, MouseEvent>) => unknown;
  onMouseLeave?: (event: DOMEvent<T, MouseEvent>) => unknown;
  onPointerDown?: (event: DOMEvent<T, PointerEvent>) => unknown;
  onPointerMove?: (event: DOMEvent<T, PointerEvent>) => unknown;
  onPointerUp?: (event: DOMEvent<T, PointerEvent>) => unknown;
  onPointerCancel?: (event: DOMEvent<T, PointerEvent>) => unknown;
  onFocus?: (event: DOMEvent<T, FocusEvent>) => unknown;
  onBlur?: (event: DOMEvent<T, FocusEvent>) => unknown;
  onPaste?: (event: DOMEvent<T, ClipboardEvent>) => unknown;
  [name: string]: unknown;
}
export type SvgAttributes = Attributes<SVGSVGElement>;
const svgTags = new Set(['svg', 'g', 'path', 'circle', 'ellipse', 'line', 'polyline', 'polygon', 'rect', 'text', 'tspan', 'defs', 'linearGradient', 'radialGradient', 'stop', 'clipPath', 'mask', 'use']);
function nodes(content: Content): Node[] {
  if (Array.isArray(content)) return content.flatMap(nodes);
  if (content === null || content === undefined || typeof content === 'boolean') return [];
  if (content instanceof DocumentFragment) return [...content.childNodes];
  return [content instanceof Node ? content : document.createTextNode(String(content))];
}
function append(parent: Node, content: Content) { for (const node of nodes(content)) parent.appendChild(node); }

export function el<K extends keyof HTMLElementTagNameMap>(tag: K, props?: Attributes<HTMLElementTagNameMap[K]>): HTMLElementTagNameMap[K];
export function el<K extends keyof SVGElementTagNameMap>(tag: K, props?: Attributes<SVGElementTagNameMap[K]>): SVGElementTagNameMap[K];
export function el(tag: string, props?: Attributes<any>): HTMLElement;
export function el(tag: string, props: Attributes<any> = {}): any {
  const node = svgTags.has(tag) ? document.createElementNS('http://www.w3.org/2000/svg', tag) : document.createElement(tag);
  const owner = currentLifetime();
  for (const name of Object.keys(props)) {
    if (name === 'children' || name === 'ref') continue;
    if (/^on[A-Z]/.test(name)) {
      const eventName = name.slice(2).toLowerCase();
      node.addEventListener(eventName, event => inLifetime(owner, () => sample(() => {
        const handler = props[name];
        if (typeof handler === 'function') transactionEvent(() => handler(event));
      })));
      continue;
    }
    let previous: unknown;
    observe(() => {
      const value = props[name];
      if (Object.is(value, previous)) return;
      previous = value;
      if (name === 'style') {
        if (typeof value === 'string') (node as HTMLElement).style.cssText = value;
        else {
          (node as HTMLElement).removeAttribute('style');
          for (const [key, v] of Object.entries((value ?? {}) as object)) {
            (node as HTMLElement).style.setProperty(key.replace(/[A-Z]/g, c => `-${c.toLowerCase()}`), String(v));
          }
        }
      } else if (name === 'classList') {
        for (const [key, enabled] of Object.entries((value ?? {}) as object)) node.classList.toggle(key, !!enabled);
      } else if (name === 'innerHTML') {
        node.innerHTML = String(value ?? '');
      } else if (name === 'value' || name === 'checked' || name === 'selected' || name === 'indeterminate') {
        const next = name === 'value' ? value ?? '' : !!value;
        if (tag === 'option' && name === 'value') node.setAttribute('value', String(next));
        if ((node as any)[name] !== next) (node as any)[name] = next;
      } else if (value === null || value === undefined || (value === false && !name.startsWith('aria-') && !name.startsWith('data-'))) {
        node.removeAttribute(name);
      } else {
        node.setAttribute(name === 'htmlFor' ? 'for' : name, value === true && !name.startsWith('aria-') && !name.startsWith('data-') ? '' : String(value));
      }
    }, true);
  }
  append(node, sample(() => props.children));
  // A select's value can only be selected after its options are inserted.
  if (tag === 'select' && 'value' in props) {
    const sync = () => { (node as HTMLSelectElement).value = String(sample(() => props.value) ?? ''); };
    sync();
    const options = new MutationObserver(sync);
    options.observe(node, {childList: true, subtree: true, attributes: true, attributeFilter: ['value']});
    cleanup(() => options.disconnect());
  }
  if (props.ref) sample(() => props.ref!(node));
  return node;
}
function transactionEvent(fn: () => unknown) { fn(); flush(); }

interface Region { fragment: DocumentFragment; start: Comment; end: Comment }
function region(): Region {
  const start = document.createComment('lh');
  const end = document.createComment('/lh');
  const fragment = document.createDocumentFragment(); fragment.append(start, end);
  return { fragment, start, end };
}
function clear(region: Region) {
  while (region.start.nextSibling && region.start.nextSibling !== region.end) region.start.nextSibling.remove();
}
function insert(region: Region, content: Content) {
  const parent = region.end.parentNode;
  if (parent) for (const node of nodes(content)) parent.insertBefore(node, region.end);
}
function allNodes(region: Region): Node[] {
  const result: Node[] = [];
  let node: Node | null = region.start;
  while (node) { result.push(node); if (node === region.end) break; node = node.nextSibling; }
  return result;
}

// A text or DOM expression owns its bindings until the expression changes.
export function live(read: () => Content): DocumentFragment {
  const place = region();
  let previous: Content;
  observe(() => {
    const content = read();
    if (Object.is(content, previous)) return;
    previous = content;
    if ((typeof content === 'string' || typeof content === 'number') && place.start.nextSibling instanceof Text && place.start.nextSibling.nextSibling === place.end) {
      place.start.nextSibling.textContent = String(content);
    } else { clear(place); insert(place, content); }
  }, true);
  return place.fragment;
}
export function When<T>(props: {when: T; fallback?: Content; children?: Content | ((value: NonNullable<T>) => Content); keyed: true}): DocumentFragment;
export function When<T>(props: {when: T; fallback?: Content; children?: Content | ((value: Read<NonNullable<T>>) => Content); keyed?: false}): DocumentFragment;
export function When<T>(props: { when: T; fallback?: Content; children?: Content | ((value: Read<NonNullable<T>>) => Content); keyed?: boolean }): DocumentFragment {
  const place = region();
  const owner = currentLifetime();
  let shown: unknown = Symbol();
  let dispose: (() => void) | undefined;
  observe(() => {
    const value = props.when;
    const next = props.keyed ? value : !!value;
    if (Object.is(next, shown)) return;
    shown = next;
    dispose?.(); clear(place);
    inLifetime(owner, () => sample(() => scope(remove => {
      dispose = remove;
      const child = value ? props.children : live(() => props.fallback);
      insert(place, typeof child === 'function' ? child((props.keyed ? value : () => props.when) as Read<NonNullable<T>>) : child);
    })));
  }, true);
  cleanup(() => dispose?.());
  return place.fragment;
}
export function Repeat<T>(props: { each: readonly T[] | null | undefined | false; children: (item: T, index: Read<number>) => Content; fallback?: Content }): DocumentFragment {
  const place = region();
  const owner = currentLifetime();
  type Row = { place: Region; dispose: () => void; index: (index: number) => unknown };
  let rows = new Map<T, Row[]>();
  let empty: (() => void) | undefined;
  observe(() => {
    const values = props.each || [];
    sample(() => {
      if (empty) { empty(); empty = undefined; clear(place); }
      const next = new Map<T, Row[]>();
      const ordered: Row[] = [];
      values.forEach((value, i) => {
        let row = rows.get(value)?.shift();
        if (!row) inLifetime(owner, () => scope(dispose => {
          const [index, setIndex] = cell(i);
          const item = region(); insert(item, props.children(value, index));
          row = { place: item, dispose, index: setIndex };
        }));
        row!.index(i);
        next.set(value, [...(next.get(value) ?? []), row!]); ordered.push(row!);
      });
      for (const rest of rows.values()) for (const row of rest) { row.dispose(); for (const node of allNodes(row.place)) node.parentNode?.removeChild(node); }
      rows = next;
      let cursor = place.start.nextSibling;
      for (const row of ordered) for (const node of allNodes(row.place)) {
        if (cursor === node) cursor = cursor.nextSibling;
        else place.end.parentNode?.insertBefore(node, cursor ?? place.end);
      }
      if (!values.length && 'fallback' in props) inLifetime(owner, () => scope(dispose => { empty = dispose; insert(place, props.fallback); }));
    });
  }, true);
  cleanup(() => { empty?.(); for (const rest of rows.values()) for (const row of rest) row.dispose(); });
  return place.fragment;
}
export function Overlay(props: { children?: Content; mount?: HTMLElement }): Comment {
  const container = el('div', { get children() { return props.children; } });
  (props.mount ?? document.body).append(container);
  cleanup(() => container.remove());
  return document.createComment('overlay');
}

// The Zest host owns all DOM bindings, requests, and subscriptions below it.
// Child views use native DOM, so forms, SVG, and existing CSS keep their semantics.
export class LonghouseView extends BaseComponent {
  view: () => Content = () => null;
  connectedCallback() {
    super.connectedCallback();
    scope(dispose => { this.own(dispose); this.replaceChildren(...nodes(this.view())); });
    flush();
  }
  disconnectedCallback() { super.disconnectedCallback(); this.replaceChildren(); }
}
customElements.define('longhouse-view', LonghouseView);
export function mount(view: () => Content, target: HTMLElement): () => void {
  const host = document.createElement('longhouse-view') as LonghouseView;
  host.style.display = 'contents'; host.view = view; target.append(host);
  return () => host.remove();
}

export function defineView<P>(name: string, render: (props: P) => Content): (props: P) => LonghouseView {
  if (!customElements.get(name)) customElements.define(name, class extends LonghouseView {});
  return props => {
    const host = document.createElement(name) as LonghouseView;
    host.setAttribute('data-longhouse-view', '');
    host.view = () => render(props);
    return host;
  };
}
