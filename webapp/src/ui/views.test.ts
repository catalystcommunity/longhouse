import { describe, expect, it, vi } from 'vitest';
import { BaseComponent } from '@catalystcommunity/base-components';
import { cell, cleanup, computed, observe, query, scope, transaction } from './state';
import { el, live, mount, Overlay, Repeat, When } from './dom';
import { fireEvent, render, waitFor } from './testing';

const deferred = <T>() => { let resolve!: (value: T) => void; let reject!: (error: unknown) => void; const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no; }); return { promise, resolve, reject }; };

describe('Zest view ownership', () => {
  it('selects an async option and keeps a blank placeholder value', async () => {
    const [rows, setRows] = cell<string[]>([]);
    const [selected, setSelected] = cell('b');
    const screen = render(() => el('select', {
      'aria-label': 'Member',
      get value() { return selected(); },
      onChange: event => setSelected(event.currentTarget.value),
      get children() { return [
        el('option', {value: '', children: 'Choose a member'}),
        Repeat({get each() { return rows(); }, children: id => el('option', {value: id, children: id})}),
      ]; },
    }));
    const select = screen.getByRole('combobox') as HTMLSelectElement;
    setRows(['a', 'b']);
    await waitFor(() => expect(select.value).toBe('b'));
    fireEvent.change(select, {target: {value: ''}});
    expect(selected()).toBe('');
    expect(select.options[0].value).toBe('');
    setRows(['a', 'b', 'c']);
    await waitFor(() => expect(select.value).toBe(''));
    expect(select.selectedIndex).toBe(0);
  });
  it('keeps focused inputs and keyed rows while other rows load', () => {
    const first = {id: 'a'}, second = {id: 'b'};
    const [rows, setRows] = cell([first]);
    const [count, setCount] = cell(0);
    const screen = render(() => el('section', { get children() { return [
      live(() => `Loaded ${count()}`),
      Repeat({get each() {return rows();}, children: row => el('input', {'aria-label': row.id})}),
    ]; } }));
    const input = screen.getByRole('textbox', {name:'a'}) as HTMLInputElement;
    input.focus(); fireEvent.input(input, {target:{value:'unsaved edit'}}); input.setSelectionRange(2, 5);
    transaction(() => { setRows([first, second]); setCount(2); });
    expect(screen.getByRole('textbox', {name:'a'})).toBe(input);
    expect(input.value).toBe('unsaved edit'); expect(document.activeElement).toBe(input);
    expect(input.selectionStart).toBe(2); expect(screen.getByText('Loaded 2')).toBeInTheDocument();
    setRows([first]); expect(document.activeElement).toBe(input);
  });
  it('releases subscriptions, timers, and overlays when a Zest host disconnects', async () => {
    const [value, setValue] = cell(0);
    const effect = vi.fn(), disposed = vi.fn();
    const screen = render(() => {
      cleanup(disposed); observe(() => effect(value()));
      return Overlay({children: el('p', {children:'Owned overlay'})});
    });
    expect(screen.container.firstElementChild).toBeInstanceOf(BaseComponent);
    await waitFor(() => expect(effect).toHaveBeenCalledTimes(1));
    screen.unmount(); setValue(1);
    expect(disposed).toHaveBeenCalledTimes(1); expect(effect).toHaveBeenCalledTimes(1);
    expect(document.body.textContent).not.toContain('Owned overlay');
  });
  it('runs a new child lifetime when a conditional opens again', () => {
    const [open, setOpen] = cell(true); const disposed = vi.fn();
    const screen=render(() => When({get when(){return open();},get children(){ cleanup(disposed); return el('input', {'aria-label':'draft'}); }}));
    const input=screen.getByRole('textbox'); setOpen(false); expect(disposed).toHaveBeenCalledTimes(1);
    setOpen(true); expect(screen.getByRole('textbox')).not.toBe(input);
    screen.unmount(); expect(disposed).toHaveBeenCalledTimes(2);
  });
  it('refreshes a conditional fallback without duplicating its nodes', () => {
    const [rows,setRows]=cell<string[]>([]); const [label,setLabel]=cell('Empty');
    const screen=render(() => Repeat({get each(){return rows();},children: text=>text,get fallback(){return live(label);}}));
    setLabel('Nothing'); expect(screen.container.textContent).toBe('Nothing');
    setRows(['A']); expect(screen.container.textContent).toBe('A');
    setRows([]); expect(screen.container.textContent).toBe('Nothing');
  });
});

describe('requests and derived state', () => {
  it('keeps a request when a URL changes but its resource ID stays equal', async () => {
    const [url, setURL] = cell('/projects/a?tab=tasks');
    const fetch = vi.fn(async (id: string) => id);
    render(() => { const [data] = query(() => url().split('?')[0], fetch); return live(data); });
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(1));
    setURL('/projects/a?tab=discussion');
    expect(fetch).toHaveBeenCalledTimes(1);
    setURL('/projects/b');
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
  });
  it('ignores an old house response and a response after disconnect', async () => {
    const [house,setHouse]=cell('a'); const a=deferred<string>(),b=deferred<string>();
    let result!: ReturnType<typeof query<string>>[0];
    const screen=render(() => { [result]=query(house,h=>h==='a'?a.promise:b.promise); return live(()=>result()??'Waiting'); });
    await waitFor(()=>expect(result.loading).toBe(true)); setHouse('b');
    b.resolve('House B'); await waitFor(()=>expect(screen.container.textContent).toBe('House B'));
    a.resolve('House A'); await Promise.resolve(); expect(result()).toBe('House B');
    screen.unmount(); setHouse('a'); expect(result()).toBe('House B');
  });
  it('keeps data during refresh and exposes request errors', async () => {
    const next=deferred<string>(); let calls=0;
    let result!: ReturnType<typeof query<string>>[0], refresh!:()=>Promise<unknown>;
    const screen=render(()=>{const [data,actions]=query(async()=>++calls===1?'Saved':next.promise); result=data;refresh=actions.refetch;return live(()=>data()??'');});
    await waitFor(()=>expect(result.state).toBe('ready'));
    void refresh(); expect(result.state).toBe('refreshing'); expect(result.latest).toBe('Saved');
    next.reject(new Error('offline')); await waitFor(()=>expect(result.state).toBe('errored'));
    expect(result.error).toBeInstanceOf(Error); expect(result.latest).toBe('Saved'); screen.unmount();
  });
  it('does not reset a feed when its project IDs stay equal', () => {
    scope(dispose=>{
      const [project,setProject]=cell({id:'a',title:'First'});
      const key=computed(()=>({id:project().id}),undefined,{equals:(a,b)=>a.id===b.id});
      const first=key(); setProject({id:'a',title:'Renamed'}); expect(key()).toBe(first);
      setProject({id:'b',title:'Other'}); expect(key()).not.toBe(first); dispose();
    });
  });
});
