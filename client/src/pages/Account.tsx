import { For, Show } from "solid-js";
import { useNavigate } from "@solidjs/router";
import { AuthGate } from "~/components/AuthGate";
import { signOut, useHouses, useSession } from "~/stores/auth";

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

  const onSignOut = () => {
    signOut();
    navigate(import.meta.env.DEV ? "/dev-login" : "/");
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
