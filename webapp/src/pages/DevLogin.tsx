/**
 * Dev-only login page. Lists every (member, house) the local API exposes
 * via DevAuthService.ListDevUsers, click one to call DevAuthService.DevLogin
 * and stash the resulting JWT in the auth store.
 *
 * The whole module is dynamically imported (see App.tsx) so prod builds
 * tree-shake it out — the dead-code branch on `import.meta.env.DEV`
 * eliminates the import.
 */

import { For, Show, createResource, createSignal } from "solid-js";
import { useNavigate } from "@solidjs/router";
import { devAuthClient } from "~/data/clients";
import { finishLogin } from "~/lib/session";
import type { DevUserEntry } from "@longhouse/client";

export const DevLogin = () => {
  const navigate = useNavigate();
  const [users] = createResource(async () => {
    const res = await devAuthClient.listDevUsers({});
    return res.users ?? [];
  });
  const [busy, setBusy] = createSignal<string | null>(null);
  const [err, setErr] = createSignal<string | null>(null);

  const choose = async (u: DevUserEntry) => {
    setErr(null);
    setBusy(u.memberId);
    try {
      const resp = await devAuthClient.devLogin({ memberId: u.memberId });
      await finishLogin(resp);
      navigate("/", { replace: true });
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
      setBusy(null);
    }
  };

  return (
    <section class="dev-login reveal d1">
      <div class="dev-login-card">
        <span class="kicker plain" style="color: var(--rust); border-color: color-mix(in oklab, var(--rust) 40%, transparent); background: color-mix(in oklab, var(--rust) 14%, var(--paper))">
          DEV MODE
        </span>
        <h1>Choose a user to sign in as</h1>
        <p class="lede">
          This page only exists in development. The list comes from the local API's
          <code> devauth.ListDevUsers</code> method, which itself is only registered
          when <code>LONGHOUSE_DEV_AUTH_ENABLED=true</code> and
          <code>LONGHOUSE_ENV</code> is <code>dev</code> or <code>nonprod</code>.
        </p>

        <Show when={err()}>
          {(message) => (
            <div class="dev-login-err">
              <b>Couldn't sign in:</b> {message()}
              <p style="margin: 6px 0 0; font-size: 13px">
                Is the api running on port 6080? Is dev-auth actually enabled? Try{" "}
                <code>LONGHOUSE_DEV_AUTH_ENABLED=true LONGHOUSE_ENV=dev go run ./api serve</code>
              </p>
            </div>
          )}
        </Show>

        <Show
          when={users() && users()!.length > 0}
          fallback={
            <div class="dev-login-empty">
              <Show when={users.loading}>Loading users…</Show>
              <Show when={users.error}>
                <b>Failed to fetch users.</b> The api isn't reachable, or dev-auth isn't enabled.
              </Show>
              <Show when={users() && users()!.length === 0}>
                No members in the local DB. Sign in once with linkkeys (or seed the DB) so there's a member to impersonate.
              </Show>
            </div>
          }
        >
          <ul class="dev-login-list">
            <For each={users()}>
              {(u) => (
                <li>
                  <button
                    disabled={busy() !== null}
                    onClick={() => choose(u)}
                    class={busy() === u.memberId ? "busy" : ""}
                  >
                    <span class="who-name">
                      {u.displayName ?? u.linkkeysUserId ?? u.memberId}
                    </span>
                    <span class="meta">
                      <span class="house">{u.houseName}</span>
                      <Show when={u.linkkeysDomain && u.linkkeysUserId}>
                        <span class="dot" />
                        <span class="ident">{u.linkkeysUserId}@{u.linkkeysDomain}</span>
                      </Show>
                      <Show when={u.roles.length > 0}>
                        <span class="dot" />
                        <span class="roles">{u.roles.join(", ")}</span>
                      </Show>
                    </span>
                  </button>
                </li>
              )}
            </For>
          </ul>
        </Show>
      </div>
    </section>
  );
};
