import type { ParentComponent } from "solid-js";
import { Header } from "~/components/Header";
import { Tabbar } from "~/components/Tabbar";

// Injected by vite.config.ts from version/VERSION.txt — the repo's source
// of truth for the release version, owned by CI.
declare const __APP_VERSION__: string;

export const AppShell: ParentComponent = (props) => (
  <>
    <div class="shell">
      <Header />
      {props.children}
      <footer class="foot">
        <span>Longhouse &middot; v{__APP_VERSION__}</span>
        <a
          href="https://github.com/catalystcommunity/longhouse"
          target="_blank"
          rel="noopener noreferrer"
        >
          source
        </a>
      </footer>
    </div>
    <Tabbar />
  </>
);
