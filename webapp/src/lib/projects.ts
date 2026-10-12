/**
 * Project scoping helpers shared by the dashboard, the Projects page, and
 * the task-side "Add to project" control.
 *
 * The pure helpers decide "is this my project" and "which tasks are in no
 * project". The async loaders fan out over a project list with the
 * project client passed in (so tests can hand in a fake). Each loader is
 * tolerant of per-project failures: one project's call failing drops only
 * that project, never the whole result.
 */

import type { Member, Project, ProjectScopedListRequest, Task, TaskList } from "@longhouse/client";
import { isTaskClosed } from "./derive";

/** Page size for the fan-out list calls. The server default is 50 and its
 *  cap is 500; ask for the cap so a busy project isn't silently truncated. */
export const PROJECT_LIST_LIMIT = 500;

/** Explicit owners + members of a project. */
export interface ProjectGrants {
  ownerIds: string[];
  memberIds: string[];
}

/** A project is mine when I created it (the created_by fallback owner), or
 *  I'm an explicit owner, or an explicit member. */
export const isMyProject = (p: Project, me: string, grants?: ProjectGrants): boolean =>
  (!!p.createdByMemberId && p.createdByMemberId === me) ||
  (grants?.ownerIds.includes(me) ?? false) ||
  (grants?.memberIds.includes(me) ?? false);

/** Union of task ids across the given projects' task lists. */
export const taskIdsInProjects = (
  tasksByProject: Map<string, ProjectTasks>,
  projectIds?: Iterable<string>,
): Set<string> => {
  const out = new Set<string>();
  const ids = projectIds ? Array.from(projectIds) : Array.from(tasksByProject.keys());
  for (const pid of ids) {
    for (const t of tasksByProject.get(pid)?.tasks ?? []) out.add(t.taskId);
  }
  return out;
};

/** Tasks that belong to no project ("miscellaneous" work). */
export const tasksInNoProject = (tasks: Task[], inAnyProject: Set<string>): Task[] =>
  tasks.filter((t) => !inAnyProject.has(t.taskId));

/** Open (not done, not cancelled, not deleted) task count. */
export const openTaskCount = (tasks: Task[]): number =>
  tasks.filter((t) => !t.deletedAt && !isTaskClosed(t)).length;

/** Project ids that hold the given task. */
export const projectsContainingTask = (
  tasksByProject: Map<string, ProjectTasks>,
  taskId: string,
): Set<string> => {
  const out = new Set<string>();
  for (const [pid, pt] of tasksByProject) {
    if (pt.tasks.some((t) => t.taskId === taskId)) out.add(pid);
  }
  return out;
};

// ---- Projects page filter ----------------------------------------------

export type ProjectsFilter = "all" | "mine";

const filterKey = (memberId: string) => `longhouse.projects.filter.${memberId}`;

/** The viewer's remembered All/Mine choice, or undefined when none is saved
 *  (or storage is unavailable — private windows, blocked site data). */
export const readProjectsFilter = (memberId: string): ProjectsFilter | undefined => {
  try {
    const v = localStorage.getItem(filterKey(memberId));
    return v === "all" || v === "mine" ? v : undefined;
  } catch {
    return undefined;
  }
};

export const writeProjectsFilter = (memberId: string, v: ProjectsFilter): void => {
  try {
    localStorage.setItem(filterKey(memberId), v);
  } catch { /* storage is a convenience only */ }
};

/** Effective filter: the saved choice wins; otherwise Mine when the viewer
 *  has any projects, else All. */
export const effectiveProjectsFilter = (
  saved: ProjectsFilter | undefined,
  myCount: number,
): ProjectsFilter => saved ?? (myCount > 0 ? "mine" : "all");

// ---- async loaders ------------------------------------------------------

/** The slice of ProjectAsyncClient the loaders call. */
export interface ProjectGrantsClient {
  listProjectMembers(req: string): Promise<Member[]>;
  listProjectOwners(req: string): Promise<Member[]>;
}
export interface ProjectTasksClient {
  listProjectTasks(req: ProjectScopedListRequest): Promise<TaskList>;
}

/** Visible tasks of one project, plus how many the server withheld. The
 *  next free position for an append is `tasks.length + hiddenCount`. */
export interface ProjectTasks {
  tasks: Task[];
  hiddenCount: number;
}

/** Ids of the projects I own or belong to. The created_by check needs no
 *  call; the explicit owner/member lists are fetched per project. */
export const loadMyProjectIds = async (
  client: ProjectGrantsClient,
  projects: Project[],
  me: string,
): Promise<Set<string>> => {
  const mine = new Set<string>(
    projects.filter((p) => isMyProject(p, me)).map((p) => p.projectId),
  );
  await Promise.all(
    projects
      .filter((p) => !mine.has(p.projectId))
      .map(async (p) => {
        try {
          const [pm, po] = await Promise.all([
            client.listProjectMembers(p.projectId),
            client.listProjectOwners(p.projectId),
          ]);
          const grants: ProjectGrants = {
            ownerIds: po.map((m) => m.memberId),
            memberIds: pm.map((m) => m.memberId),
          };
          if (isMyProject(p, me, grants)) mine.add(p.projectId);
        } catch { /* one project's grants failing shouldn't drop the rest */ }
      }),
  );
  return mine;
};

/** Task lists for every given project, keyed by project id. A project whose
 *  call fails is absent from the map. */
export const loadProjectTasks = async (
  client: ProjectTasksClient,
  houseId: string,
  projects: Project[],
): Promise<Map<string, ProjectTasks>> => {
  const out = new Map<string, ProjectTasks>();
  await Promise.all(
    projects.map(async (p) => {
      try {
        const res = await client.listProjectTasks({
          houseId,
          projectId: p.projectId,
          limit: PROJECT_LIST_LIMIT,
        });
        out.set(p.projectId, { tasks: res.tasks, hiddenCount: Number(res.hiddenCount ?? 0) });
      } catch { /* tolerate */ }
    }),
  );
  return out;
};
