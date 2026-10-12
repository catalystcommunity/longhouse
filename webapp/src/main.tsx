/* @refresh reload */
import { render } from "solid-js/web";
import "./styles/index.css";
import { App } from "./App";
import { startTokenRefresh } from "~/lib/refreshTimer";
import { loadHouses, resumeSession } from "~/lib/session";
import { setRestoring, useSession } from "~/stores/auth";

const root = document.getElementById("root");
if (!root) throw new Error("missing #root in index.html");

// No usable bearer at boot (first visit, or the stored one expired while the
// app was closed): try the browser session cookie before showing the
// sign-in card. This is what keeps a person signed in between launches.
// The login callback is skipped: it replaces the session, and a resume racing
// it could leave the browser holding the replaced session's cookie.
if (!useSession()() && window.location.pathname !== "/auth/callback") {
  setRestoring(true);
  void resumeSession().finally(() => setRestoring(false));
}

// Background-refresh the token when it's nearing expiry, so role/membership
// changes take effect inside the token's staleness window without forcing a
// full re-login. No-op until the user has a session.
startTokenRefresh();

// Heal the in-memory house list from /me on every cold start. localStorage
// holds the houses across reloads, but the schema can drift (e.g. a new
// field added to HouseSummary) — refreshing once on boot keeps the local
// snapshot in step with the api's current shape without forcing the user
// to sign in again.
if (useSession()()) {
  void loadHouses().catch(() => {
    // tolerated — a stale snapshot is better than crashing on boot. The
    // next protected request will surface any real auth problem.
  });
}

render(() => <App />, root);
