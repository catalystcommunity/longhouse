import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, waitFor } from '~/ui/testing';
import { authClient } from '~/data/clients';
import { signOut } from '~/stores/auth';
import { SignInPage } from './SignIn';

vi.mock('~/data/clients', () => ({ authClient: { beginBrowserLogin: vi.fn() } }));
beforeEach(() => { localStorage.removeItem('longhouse.login'); vi.clearAllMocks(); });

describe('email sign-in', () => {
  it('saves the login across page visits and sign-out, and shows service errors', async () => {
    vi.mocked(authClient.beginBrowserLogin).mockRejectedValue(new Error("can't sign in with that domain"));
    const screen = render(SignInPage);
    fireEvent.input(screen.getByLabelText('Sign in with another domain'), { target: { value: 'foo@example.com' } });
    fireEvent.submit(screen.getByRole('button', { name: 'Continue' }).closest('form')!);
    await waitFor(() => expect(screen.getByRole('alert').textContent).toBe("can't sign in with that domain"));
    expect(authClient.beginBrowserLogin).toHaveBeenCalledWith({ login: 'foo@example.com' });
    signOut(); screen.unmount();
    const next = render(SignInPage);
    expect((next.getByLabelText('Sign in with another domain') as HTMLInputElement).value).toBe('foo@example.com');
  });

  it('keeps home sign-in separate from the saved login', async () => {
    localStorage.setItem('longhouse.login', 'foo@example.com');
    vi.mocked(authClient.beginBrowserLogin).mockRejectedValue(new Error('identity service unavailable'));
    const screen = render(SignInPage);
    fireEvent.click(screen.getByRole('button', { name: 'Sign in with the home domain' }));
    await waitFor(() => expect(screen.getByRole('alert').textContent).toBe('identity service unavailable'));
    expect(authClient.beginBrowserLogin).toHaveBeenCalledWith({});
    expect(localStorage.getItem('longhouse.login')).toBe('foo@example.com');
  });

  it('prevents duplicate submissions while discovery is pending', async () => {
    let reject!: (reason: Error) => void;
    vi.mocked(authClient.beginBrowserLogin).mockImplementation(() => new Promise((_, no) => { reject = no; }));
    const screen = render(SignInPage);
    const form = screen.getByRole('button', { name: 'Continue' }).closest('form')!;
    fireEvent.submit(form); fireEvent.submit(form);
    expect(authClient.beginBrowserLogin).toHaveBeenCalledTimes(1);
    expect(screen.getByRole('button', { name: 'Starting sign-in…' }).hasAttribute('disabled')).toBe(true);
    reject(new Error('that domain has no public LinkKeys sign-in service'));
    await waitFor(() => expect(screen.getByRole('alert').textContent).toContain('no public LinkKeys'));
    expect(screen.getByRole('button', { name: 'Continue' }).hasAttribute('disabled')).toBe(false);
  });
});
