import { For, Show, createResource, createSignal } from "solid-js";
import { useNavigate } from "@solidjs/router";
import { AuthGate } from "~/components/AuthGate";
import { signOut, useHouses, useSession } from "~/stores/auth";
import { authClient } from "~/data/clients";

/**
 * Per-user account surface. Today this is a read-only view of the signed-in
 * identity, the houses the bearer can act in (with the roles in each), and
 * a sign-out button. As user-scoped settings show up in CSIL, they land
 * here.
 */
export const AccountPage = () => {
  const navigate = useNavigate();
  const session = useSession();
  const houses = useHouses();
  const [revoking, setRevoking] = createSignal<string | null>(null);
  const [sessionError, setSessionError] = createSignal<string | null>(null);
  const [cliSessions, { refetch: refetchCliSessions }] = createResource(
    () => session()?.token,
    async () => (await authClient.listSessions({})).sessions ?? [],
  );

  const onSignOut = () => {
    signOut();
    navigate(import.meta.env.DEV ? "/dev-login" : "/");
  };

  const revokeCliSession = async (sessionId: string) => {
    setRevoking(sessionId);
    setSessionError(null);
    try {
      await authClient.revokeSession({ sessionId });
      await refetchCliSessions();
    } catch (error) {
      setSessionError(error instanceof Error ? error.message : String(error));
    } finally {
      setRevoking(null);
    }
  };

  return (
    <AuthGate>
      <div class="section-hd reveal">
        <h2>Account <em>you, and your houses</em></h2>
        <p class="lead">Your identity comes from linkkeys; per-house roles come from the bearer.</p>
      </div>

      <section style="margin-top:16px;padding:20px 22px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);box-shadow:var(--shadow-low)">
        <h3 style="margin:0 0 10px;font-family:var(--display);font-size:20px;color:var(--grass-4)">
          Identity
        </h3>
        <Show when={session()} fallback={<p style="color:var(--ink-mute)">Not signed in.</p>}>
          {(s) => (
            <dl style="display:grid;grid-template-columns:max-content 1fr;gap:8px 18px;margin:0;font-size:14px">
              <dt style="color:var(--ink-mute)">Display name</dt>
              <dd style="margin:0">{s().displayName || "—"}</dd>
              <dt style="color:var(--ink-mute)">User id</dt>
              <dd style="margin:0;font-family:var(--mono,monospace);font-size:13px">{s().userId}</dd>
              <dt style="color:var(--ink-mute)">Domain</dt>
              <dd style="margin:0;font-family:var(--mono,monospace);font-size:13px">{s().domain}</dd>
              <dt style="color:var(--ink-mute)">Token expires</dt>
              <dd style="margin:0">{new Date(s().expiresAt).toLocaleString()}</dd>
            </dl>
          )}
        </Show>
      </section>

      <section style="margin-top:20px;padding:20px 22px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);box-shadow:var(--shadow-low)">
        <h3 style="margin:0 0 10px;font-family:var(--display);font-size:20px;color:var(--grass-4)">
          CLI sessions
        </h3>
        <p style="margin:0 0 14px;color:var(--ink-mute);font-size:14px">
          Revoke a command-line session that you no longer use. Its current access bearer can work until it expires.
        </p>
        <Show when={sessionError()}>{(message) => <p class="err">{message()}</p>}</Show>
        <Show when={!cliSessions.loading} fallback={<p style="color:var(--ink-mute)">Loading CLI sessions…</p>}>
          <Show when={(cliSessions() ?? []).length > 0} fallback={<p style="color:var(--ink-mute)">No CLI sessions.</p>}>
            <ul style="list-style:none;padding:0;margin:0;display:flex;flex-direction:column;gap:8px">
              <For each={cliSessions()}>
                {(cliSession) => {
                  const inactive = () => Boolean(cliSession.revokedAt) || Date.parse(cliSession.expiresAt) <= Date.now();
                  return (
                    <li style="display:flex;justify-content:space-between;align-items:center;gap:12px;padding:10px 12px;border:1px solid var(--line);border-radius:var(--r-md)">
                      <span>
                        <strong>{cliSession.clientName}</strong>
                        <span style="display:block;color:var(--ink-mute);font-size:12px">
                          {inactive() ? "Inactive" : `Last used ${new Date(cliSession.lastUsedAt).toLocaleString()}`}
                        </span>
                      </span>
                      <button
                        class="btn btn-ghost"
                        type="button"
                        disabled={inactive() || revoking() !== null}
                        onClick={() => void revokeCliSession(cliSession.sessionId)}
                      >
                        {revoking() === cliSession.sessionId ? "Revoking…" : "Revoke"}
                      </button>
                    </li>
                  );
                }}
              </For>
            </ul>
          </Show>
        </Show>
      </section>

      <section style="margin-top:20px;padding:20px 22px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);box-shadow:var(--shadow-low)">
        <h3 style="margin:0 0 10px;font-family:var(--display);font-size:20px;color:var(--grass-4)">
          Houses
        </h3>
        <Show when={houses().length > 0} fallback={<p style="color:var(--ink-mute)">No houses yet.</p>}>
          <ul style="list-style:none;padding:0;margin:0;display:flex;flex-direction:column;gap:8px">
            <For each={houses()}>
              {(h) => (
                <li style="display:flex;justify-content:space-between;gap:12px;padding:10px 12px;border:1px solid var(--line);border-radius:var(--r-md)">
                  <span style="font-weight:500">{h.name || h.id}</span>
                  <span style="color:var(--ink-mute);font-size:13px">
                    {h.roles?.length ? h.roles.join(", ") : "member"}
                  </span>
                </li>
              )}
            </For>
          </ul>
        </Show>
      </section>

      <section style="margin-top:20px;padding:20px 22px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);box-shadow:var(--shadow-low)">
        <h3 style="margin:0 0 10px;font-family:var(--display);font-size:20px;color:var(--grass-4)">
          Sign out
        </h3>
        <p style="margin:0 0 14px;color:var(--ink-mute);font-size:14px">
          Drops your bearer locally. The next sign-in goes through linkkeys (or the dev picker, in dev builds).
        </p>
        <button class="btn btn-primary" type="button" onClick={onSignOut}>
          Sign out
        </button>
      </section>
    </AuthGate>
  );
};
