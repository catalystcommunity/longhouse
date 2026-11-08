import { describe, expect, it } from 'vitest';
import { el, live } from './dom';
import { cleanup, cell } from './state';
import { navigation, outlet, routeParams, searchState, locationState, resetLocation } from './router';
import { fireEvent, render, waitFor } from './testing';

describe('Longhouse Zest routing', () => {
  it('preserves a page draft on query changes and disposes it on a route change', () => {
    resetLocation('/calendar?event=one');
    let mounts=0, disposals=0;
    const Calendar=()=>{mounts++;cleanup(()=>disposals++);const [params]=searchState();return el('section',{get children(){return [el('input',{'aria-label':'Draft'}),live(()=>params.event??'none')];}});};
    const screen=render(()=>outlet([{path:'/calendar',page:Calendar},{path:'/tasks',page:()=> 'Tasks'}],()=> 'Missing'));
    const input=screen.getByRole('textbox') as HTMLInputElement; input.value='Keep this';
    navigation()('/calendar?event=two',{replace:true});
    expect(screen.getByText('two')).toBeInTheDocument();expect(screen.getByRole('textbox')).toBe(input);expect(input.value).toBe('Keep this');expect(mounts).toBe(1);
    navigation()('/tasks');expect(screen.getByText('Tasks')).toBeInTheDocument();expect(disposals).toBe(1);
  });
  it('tracks project parameters, repeated query keys, back navigation, and aliases', async () => {
    resetLocation('/projects/first');
    const screen=render(()=>outlet([{path:'/projects',page:()=>{const params=routeParams<{slug:string}>(); return live(()=>params.slug);}}],()=> 'Missing'));
    expect(screen.getByText('first')).toBeInTheDocument();navigation()('/projects/second');expect(screen.getByText('second')).toBeInTheDocument();
    history.replaceState({},'', '/projects/first');window.dispatchEvent(new PopStateEvent('popstate'));expect(screen.getByText('first')).toBeInTheDocument();
    navigation()('/projects-other');expect(screen.getByText('Missing')).toBeInTheDocument();
  });
  it('keeps external, download, modified, and API links as native navigation', () => {
    resetLocation('/');
    const screen=render(()=>outlet([{path:'/',page:()=>el('div',{children:[
      el('a',{href:'/tasks',children:'Internal'}),el('a',{href:'/api/v1/auth/start',children:'Sign in'}),
      el('a',{href:'/download',download:'file',children:'Download'}),el('a',{href:'/external',rel:'external',children:'External'}),
    ]})},{path:'/tasks',page:()=> 'Task route'}],()=> 'Missing'));
    for(const name of ['Sign in','Download','External']) {const event=new MouseEvent('click',{bubbles:true,cancelable:true});screen.getByText(name).dispatchEvent(event);expect(event.defaultPrevented).toBe(false);}
    const modified=new MouseEvent('click',{bubbles:true,cancelable:true,ctrlKey:true});screen.getByText('Internal').dispatchEvent(modified);expect(modified.defaultPrevented).toBe(false);
    fireEvent.click(screen.getByText('Internal'));expect(screen.getByText('Task route')).toBeInTheDocument();
  });
});
