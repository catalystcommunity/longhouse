import { A, useParams } from "@solidjs/router";
import { For, Show, createResource } from "solid-js";
import { AvatarStack } from "~/components/Avatar";
import { useRepo } from "~/data/RepoContext";
import { memberById } from "~/data/mocks";

export const ProjectDetail = () => {
  const repo = useRepo();
  const params = useParams<{ slug: string }>();
  const [project] = createResource(() => repo.getProject(params.slug));

  return (
    <Show
      when={project()}
      fallback={
        <div style="padding:80px 0;text-align:center;color:var(--ink-mute)">
          <p>Project not found.</p>
          <p><A href="/projects" style="color:var(--ocean-2)">Back to projects</A></p>
        </div>
      }
    >
      {(p) => {
        const members = () =>
          p().members
            .map((id) => memberById.get(id))
            .filter((m): m is NonNullable<typeof m> => Boolean(m));
        const owners = () => p().owners.map((id) => memberById.get(id)?.name ?? id).join(", ");

        return (
          <>
            <div class="divider">
              <span class="rule" />
              <span class="label">project detail</span>
              <span class="rule" />
            </div>

            <article class="room-detail reveal">
              <div class="room-banner">
                <span class="banner-tag">{p().category} · long-running</span>
              </div>

              <div class="room-meta">
                <div class="room-title">
                  <div class="crumbs">
                    <A href="/projects">Projects</A>
                    <i />
                    {p().category}
                    <i />
                    <span>{p().title}</span>
                  </div>
                  <h2>
                    {p().title}, <em>second wing</em>
                  </h2>
                  <p class="lede">{p().description ?? p().blurb}</p>
                </div>

                <aside class="facts">
                  <dl>
                    <dt>Owners</dt>
                    <dd>{owners()}</dd>
                    <dt>Members</dt>
                    <dd>
                      <AvatarStack members={members()} />
                    </dd>
                    <dt>Started</dt>
                    <dd>{p().startedLabel}</dd>
                    <dt>Due</dt>
                    <dd>{p().dueLabel}</dd>
                    <Show when={p().sharedWith}>
                      {(s) => (<>
                        <dt>Shared with</dt>
                        <dd>{s()}</dd>
                      </>)}
                    </Show>
                  </dl>
                </aside>
              </div>

              <Show when={p().milestones}>
                {(milestones) => (
                  <div class="ribbon">
                    <For each={milestones()}>
                      {(m) => (
                        <div class={`rib ${m.state}`}>
                          <span class="when">{m.when}</span>
                          <span class="what">{m.label}</span>
                        </div>
                      )}
                    </For>
                  </div>
                )}
              </Show>
            </article>
          </>
        );
      }}
    </Show>
  );
};
