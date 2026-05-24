/* @refresh reload */
import { render } from "solid-js/web";
import "./styles/index.css";
import { App } from "./App";
import { RepoProvider } from "~/data/RepoContext";
import { liveRepo } from "~/data/liveRepo";
import { startTokenRefresh } from "~/lib/refreshTimer";

const root = document.getElementById("root");
if (!root) throw new Error("missing #root in index.html");

// Background-refresh the token when it's nearing expiry, so role/membership
// changes take effect inside the token's staleness window without forcing a
// full re-login. No-op until the user has a session.
startTokenRefresh();

render(
  () => (
    <RepoProvider repo={liveRepo}>
      <App />
    </RepoProvider>
  ),
  root,
);
