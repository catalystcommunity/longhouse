/**
 * Dev-only login page. Lists every (member, house) the local API exposes
 * via GET /api/v1/dev/users, click one to POST /api/v1/dev/login and stash
 * the resulting JWT in the auth store.
 *
 * The whole module is dynamically imported (see App.tsx) so prod builds
 * tree-shake it out — the dead-code branch on `import.meta.env.DEV`
 * eliminates the import.
 */

import { For, Show, createResource, createSignal } from "solid-js";
import { useNavigate } from "@solidjs/router";
import { jsonFetch } from "~/transport/http";
import { finishLogin, type LoginResponse } from "~/lib/session";

interface DevUser {
  member_id: string;
  house_id: string;
  house_name: string;
  display_name?: string;
  linkkeys_domain?: string;
  linkkeys_user_id?: string;
  roles: string[];
}

export const DevLogin = () => {
  const navigate = useNavigate();
  const [users] = createResource(async () => {
    const res = await jsonFetch<{ users: DevUser[] }>("/api/v1/dev/users");
    return res.users;
  });
  const [busy, setBusy] = createSignal<string | null>(null);
  const [err, setErr] = createSignal<string | null>(null);

  const choose = async (u: DevUser) => {
    setErr(null);
    setBusy(u.member_id);
    try {
      const resp = await jsonFetch<LoginResponse>("/api/v1/dev/login", {
        method: "POST",
        body: JSON.stringify({ member_id: u.member_id }),
      });
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
          <code> /api/v1/dev/users</code> endpoint, which itself is only registered
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
                    class={busy() === u.member_id ? "busy" : ""}
                  >
                    <span class="who-name">
                      {u.display_name ?? u.linkkeys_user_id ?? u.member_id}
                    </span>
                    <span class="meta">
                      <span class="house">{u.house_name}</span>
                      <Show when={u.linkkeys_domain && u.linkkeys_user_id}>
                        <span class="dot" />
                        <span class="ident">{u.linkkeys_user_id}@{u.linkkeys_domain}</span>
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
