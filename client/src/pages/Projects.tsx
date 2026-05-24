import { A } from "@solidjs/router";
import { For, createResource } from "solid-js";
import { useRepo } from "~/data/RepoContext";
import type { Project } from "~/data/types";

export const ProjectsPage = () => {
  const repo = useRepo();
  const [projects] = createResource(() => repo.listProjects());
  return (
    <>
      <div class="section-hd reveal">
        <h2>
          Projects <em>in flight</em>
        </h2>
        <p class="lead">Longer pieces of work with their own members, tasks, and discussion.</p>
      </div>

      <div class="rooms">
        <For each={projects() ?? []}>{(p) => <ProjectCard p={p} delay={2 + (parseInt(p.id.slice(1)) - 1) % 4} />}</For>
      </div>
    </>
  );
};

export const ProjectCard = (props: { p: Project; delay?: number }) => (
  <A
    href={`/projects/${props.p.slug}`}
    class={`room ${props.p.swatch} reveal ${props.delay ? `d${props.delay}` : ""}`}
  >
    <span class="corner">{props.p.category}</span>
    <div class="swatch">{props.p.title.charAt(0)}</div>
    <h3>{props.p.title}</h3>
    <p>{props.p.blurb}</p>
    <div class="room-foot">
      <span>{props.p.members.length} members</span>
      <div class="progress"><i style={{ width: `${props.p.progress}%` }} /></div>
      <span>{props.p.progress}%</span>
    </div>
  </A>
);
