/**
 * /auth/callback — the route the linkkeys IDP redirects the browser to after
 * authentication. It carries the sealed `encrypted_token` in the query. We
 * POST that to /api/v1/auth/complete (the api decrypts + verifies it and
 * returns our session token in the body), store the session, then redirect
 * into the app. The session token never travels in a URL — only the sealed
 * IDP token does, and that's safe (it's encrypted IDP↔RP).
 */

import { Show, createSignal, onMount } from "solid-js";
import { useNavigate, useSearchParams } from "@solidjs/router";
import { jsonFetch } from "~/transport/http";
import { finishLogin, type LoginResponse } from "~/lib/session";

export const AuthCallback = () => {
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const [error, setError] = createSignal<string | null>(null);

  onMount(async () => {
    const token = params.encrypted_token;
    if (typeof token !== "string" || token === "") {
      setError("The identity provider didn't return a token.");
      return;
    }
    try {
      const resp = await jsonFetch<LoginResponse>("/api/v1/auth/complete", {
        method: "POST",
        body: JSON.stringify({ encrypted_token: token }),
      });
      await finishLogin(resp);
      navigate("/", { replace: true });
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  });

  return (
    <section class="auth-callback reveal d1">
      <Show
        when={error()}
        fallback={
          <div class="auth-callback-card">
            <div class="spinner" aria-hidden="true" />
            <p>Signing you in…</p>
          </div>
        }
      >
        {(message) => (
          <div class="auth-callback-card">
            <h1>Sign-in failed</h1>
            <p class="err">{message()}</p>
            <a class="btn btn-primary" href="/api/v1/auth/start">Try again</a>
          </div>
        )}
      </Show>
    </section>
  );
};
