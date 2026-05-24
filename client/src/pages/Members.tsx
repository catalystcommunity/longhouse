import { For, createResource } from "solid-js";
import { useRepo } from "~/data/RepoContext";

export const MembersPage = () => {
  const repo = useRepo();
  const [members] = createResource(() => repo.listMembers());

  return (
    <>
      <div class="section-hd reveal">
        <h2>Members <em>the household</em></h2>
        <p class="lead">Everyone with access to this Longhouse instance.</p>
      </div>

      <section class="card folks reveal d1" style="margin-top:0;padding: 8px 22px 18px">
        <For each={members() ?? []}>
          {(m) => (
            <div class={`folk ${m.status === "away" ? "away" : ""}`}>
              <span class={`a lg ${m.swatch}`}>{m.initials}</span>
              <div>
                <div class="who-name">{m.name}</div>
                <div class="doing">{m.doing}</div>
              </div>
              <span class="ago">{m.lastSeenLabel}</span>
            </div>
          )}
        </For>
      </section>
    </>
  );
};
