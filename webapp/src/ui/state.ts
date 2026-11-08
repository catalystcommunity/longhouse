import BasicDataService from '@catalystcommunity/basic-data-service';

export type Read<T> = () => T;
export type Write<T> = (value: T | ((previous: T) => T)) => T;
type Dependency = { subscribe: (notify: () => void) => () => void };
export interface Lifetime { alive: boolean; releases: Set<() => void> }
let lifetime: Lifetime | undefined;
let tracking: Set<Dependency> | undefined;
let depth = 0;
let flushing = false;
const pending = new Set<() => void>();

export function currentLifetime() { return lifetime; }
export function inLifetime<T>(owner: Lifetime | undefined, fn: () => T): T {
  const previous = lifetime;
  lifetime = owner;
  try { return fn(); } finally { lifetime = previous; }
}
export function release(owner: Lifetime) {
  if (!owner.alive) return;
  owner.alive = false;
  const releases = [...owner.releases].reverse();
  owner.releases.clear();
  for (const remove of releases) remove();
}
export function scope<T>(fn: (dispose: () => void) => T): T {
  const parent = lifetime;
  const owner: Lifetime = { alive: true, releases: new Set() };
  const dispose = () => { release(owner); parent?.releases.delete(dispose); };
  parent?.releases.add(dispose);
  return inLifetime(owner, () => fn(dispose));
}
export function cleanup(remove: () => void) { lifetime?.releases.add(remove); }
export function sample<T>(fn: () => T): T {
  const previous = tracking;
  tracking = undefined;
  try { return fn(); } finally { tracking = previous; }
}
export function flush() {
  if (depth || flushing) return;
  flushing = true;
  try {
    let turns = 0;
    while (pending.size) {
      if (++turns > 1000) throw new Error('A Longhouse view repeatedly updates its own state');
      const work = [...pending]; pending.clear();
      for (const run of work) run();
    }
  } finally { flushing = false; }
}
export function transaction<T>(fn: () => T): T {
  depth++;
  try { return fn(); } finally { depth--; flush(); }
}
export function cell<T>(): [Read<T | undefined>, Write<T | undefined>];
export function cell<T>(initial: T): [Read<T>, Write<T>];
export function cell<T>(initial?: T): [Read<T | undefined>, Write<T | undefined>] {
  const data = new BasicDataService();
  data.set('state', 'value', initial);
  const dependency: Dependency = { subscribe: notify => data.subscribe(notify, { collection: 'state', keys: ['value'] }) };
  const read = () => { tracking?.add(dependency); return data.get('state', 'value') as T | undefined; };
  const write: Write<T | undefined> = value => {
    const next = typeof value === 'function' ? (value as (v: T | undefined) => T)(sample(read)) : value;
    transaction(() => data.set('state', 'value', next));
    return next;
  };
  return [read, write];
}

// Every subscription belongs to a view or an effect run. A rerun releases its
// nested subscriptions before it reads the next dependency set.
export function observe(fn: () => void, immediate = false): () => void {
  const parent = lifetime;
  let active = true;
  let runOwner: Lifetime | undefined;
  let unsubscribers: (() => void)[] = [];
  const dispose = () => {
    active = false;
    pending.delete(run);
    for (const remove of unsubscribers) remove();
    unsubscribers = [];
    if (runOwner) release(runOwner);
    parent?.releases.delete(dispose);
  };
  const notify = () => { if (active) { pending.add(run); if (!depth) flush(); } };
  const run = () => {
    if (!active || parent?.alive === false) return;
    for (const remove of unsubscribers) remove();
    unsubscribers = [];
    if (runOwner) release(runOwner);
    runOwner = { alive: true, releases: new Set() };
    const deps = new Set<Dependency>();
    const previous = tracking; tracking = deps;
    try { inLifetime(runOwner, fn); } finally { tracking = previous; }
    if (active) unsubscribers = [...deps].map(dep => dep.subscribe(notify));
  };
  parent?.releases.add(dispose);
  if (immediate) run(); else { pending.add(run); queueMicrotask(flush); }
  return dispose;
}
export function computed<T>(fn: () => T, _initial?: undefined, options?: {equals: (a: T, b: T) => boolean}): Read<T> {
  const [value, set] = cell<T>(sample(fn));
  observe(() => { const next = fn(); if (!options?.equals(sample(value), next)) set(() => next); }, true);
  return value;
}
export function afterMount(fn: () => unknown) {
  const owner = lifetime;
  queueMicrotask(() => { if (owner?.alive !== false) inLifetime(owner, () => sample(fn)); });
}

export type Query<T> = { readonly loading: boolean; readonly error: unknown } & (
  { (): T; readonly state: 'ready' | 'refreshing'; readonly latest: T } |
  { (): T | undefined; readonly state: 'unresolved' | 'pending' | 'errored'; readonly latest: T | undefined }
);
export interface QueryActions<T> { refetch: () => Promise<T | undefined>; mutate: Write<T | undefined> }
export function query<T>(fetch: () => Promise<T>): [Query<T>, QueryActions<T>];
export function query<S, T>(source: Read<S | false | null | undefined>, fetch: (source: S) => Promise<T>): [Query<T>, QueryActions<T>];
export function query<S, T>(sourceOrFetch: Read<S | false | null | undefined> | (() => Promise<T>), fetcher?: (source: S) => Promise<T>): [Query<T>, QueryActions<T>] {
  const source = fetcher ? sourceOrFetch as Read<S | false | null | undefined> : () => true as S;
  const fetch = fetcher ?? sourceOrFetch as () => Promise<T>;
  const [value, setValue] = cell<T>();
  const [loading, setLoading] = cell(false);
  const [error, setError] = cell<unknown>();
  let generation = 0;
  let active = true;
  let selected: S | false | null | undefined;
  let hasSelection = false;
  cleanup(() => { active = false; generation++; });
  const load = async (): Promise<T | undefined> => {
    const id = ++generation;
    const key = selected;
    if (key === null || key === undefined || key === false) {
      transaction(() => { setValue(undefined); setError(undefined); setLoading(false); });
      return;
    }
    transaction(() => { setError(undefined); setLoading(true); });
    try {
      const result = await fetch(key);
      if (active && generation === id) transaction(() => { setValue(() => result); setLoading(false); });
      return result;
    } catch (cause) {
      if (active && generation === id) transaction(() => { setError(() => cause); setLoading(false); });
      return undefined;
    }
  };
  observe(() => {
    const next = source();
    if (hasSelection && Object.is(next, selected)) return;
    hasSelection = true;
    selected = next;
    sample(() => { void load(); });
  });
  Object.defineProperties(value, { loading: { get: loading }, error: { get: error }, latest: { get: value }, state: { get: () => error() ? 'errored' : loading() ? value() === undefined ? 'pending' : 'refreshing' : value() === undefined ? 'unresolved' : 'ready' } });
  return [value as Query<T>, { refetch: () => sample(load), mutate: setValue }];
}
