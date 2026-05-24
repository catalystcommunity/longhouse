import type { ParentComponent } from "solid-js";
import { Header } from "~/components/Header";
import { Tabbar } from "~/components/Tabbar";

export const AppShell: ParentComponent = (props) => (
  <>
    <div class="shell">
      <Header />
      {props.children}
      <footer class="foot">
        <span>Longhouse &middot; v0.4.0</span>
        <span class="end">
          <a href="#">docs</a>
          <a href="#">trust &amp; keys</a>
          <a href="#">privacy</a>
        </span>
      </footer>
    </div>
    <Tabbar />
  </>
);
