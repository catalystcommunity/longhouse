import { authClient } from '~/data/clients';
import { el, live, When } from '~/ui/dom';
import { cell, cleanup } from '~/ui/state';
import { searchState } from '~/ui/router';

const LOGIN_KEY = 'longhouse.login';
function savedLogin(): string {
  try { return localStorage.getItem(LOGIN_KEY) ?? ''; }
  catch { return ''; }
}

export const SignInPage = () => {
  const [params] = searchState();
  const [login, setLogin] = cell(savedLogin());
  const [busy, setBusy] = cell(false);
  const [error, setError] = cell(params.error === 'start' ? 'Could not start sign-in. Try again.' : '');
  let active = true;
  cleanup(() => { active = false; });

  async function begin(useLogin: boolean) {
    if (busy()) return;
    setBusy(true);
    setError('');
    const value = login().trim();
    try {
      if (useLogin) {
        // Keep the login after sign-out. A storage failure must not block login.
        try { localStorage.setItem(LOGIN_KEY, value); } catch { /* Storage is optional. */ }
      }
      const result = await authClient.beginBrowserLogin(useLogin ? { login: value } : {});
      if (active) window.location.assign(result.redirectUrl);
    } catch (e) {
      if (active) setError(e instanceof Error ? e.message : String(e));
    } finally {
      if (active) setBusy(false);
    }
  }

  return el('section', {
    class: 'reveal d1',
    style: 'max-width:480px;margin:48px auto;padding:32px 24px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg)',
    children: [
      el('h1', { style: 'margin-top:0;color:var(--grass-4)', children: 'Sign in' }),
      el('p', { children: "Use this instance's home domain, or enter your login below." }),
      el('button', {
        type: 'button', class: 'btn btn-primary', get disabled() { return busy(); },
        onClick: () => begin(false), children: 'Sign in with the home domain',
      }),
      el('form', {
        style: 'display:flex;flex-direction:column;gap:12px;margin-top:28px',
        onSubmit: (e) => { e.preventDefault(); void begin(true); },
        children: [
          el('label', { htmlFor: 'linkkeys-login', children: 'Sign in with another domain' }),
          el('input', {
            id: 'linkkeys-login', name: 'username', type: 'email', required: true,
            autocomplete: 'username', autocapitalize: 'none', spellcheck: 'false',
            placeholder: 'name@domain.com', get value() { return login(); },
            get disabled() { return busy(); }, onInput: (e) => setLogin(e.currentTarget.value),
            style: 'width:100%;box-sizing:border-box;padding:12px;border:1px solid var(--line);border-radius:var(--r-sm);background:var(--paper);color:var(--ink)',
          }),
          el('p', { style: 'margin:0;font-size:13px;color:var(--ink-mute)', children: 'This browser remembers your login. A house on this instance must trust your domain.' }),
          el('button', {
            type: 'submit', class: 'btn btn-primary', get disabled() { return busy(); },
            children: live(() => busy() ? 'Starting sign-in…' : 'Continue'),
          }),
          When({ get when() { return error(); }, children: () => el('p', {
            class: 'err', role: 'alert', children: live(error),
          }) }),
        ],
      }),
    ],
  });
};
