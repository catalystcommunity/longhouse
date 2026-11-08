// Public interfaces used by Longhouse. Zest currently publishes JavaScript only.
declare module '@catalystcommunity/base-components' {
  export class BaseComponent extends HTMLElement {
    root: HTMLElement | ShadowRoot;
    connectedCallback(): void;
    disconnectedCallback(): void;
    own(remove: () => void): () => void;
    listen(target: EventTarget, name: string, handler: EventListener, options?: AddEventListenerOptions): () => void;
    isActive(): boolean;
    invalidate(): void;
    render(): void;
  }
  export function observeResize(element: Element, callback: (entry: ResizeObserverEntry) => void): () => void;
}
declare module '@catalystcommunity/basic-data-service' {
  export default class BasicDataService extends EventTarget {
    get(collection: string, key: string): unknown;
    set(collection: string, key: string, value: unknown): void;
    subscribe(handler: () => void, options?: {collection?: string; keys?: string[]; isActive?: () => boolean}): () => void;
  }
}
declare module '@catalystcommunity/simple-router' {
  export interface RouteData { path: string; firstPath: string; hash: string; params: Map<string, string[]> }
  export default class SimpleRouter {
    eventSender: (name: string, detail: unknown) => void;
    registerRoute(path: string, render: (route: RouteData) => void, aliases?: string[], fallback?: boolean): void;
    route(changeType?: string, state?: unknown): void;
    changePage(path: string, state?: unknown): void;
    cleanup(): void;
  }
}
declare module '@catalystcommunity/date-time-picker' {
  import { BaseComponent } from '@catalystcommunity/base-components';
  export class DateTimePicker extends BaseComponent {
    value: string;
    valueAsDate: Date | null;
    updateConfig(config: Record<string, unknown>): void;
    openPicker(): void;
    closePicker(): void;
  }
}
