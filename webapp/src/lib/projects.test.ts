import { describe, expect, it, vi } from "vitest";
import type { Member, Project, ProjectScopedListRequest, Task } from "@longhouse/client";
import {
  effectiveProjectsFilter,
  isMyProject,
  readProjectsFilter,
  writeProjectsFilter,
  loadMyProjectIds,
  loadProjectTasks,
  openTaskCount,
  projectsContainingTask,
  taskIdsInProjects,
  tasksInNoProject,
  type ProjectTasks,
} from "./projects";

const project = (projectId: string, createdByMemberId?: string): Project => ({
  projectId,
  houseId: "h1",
  name: projectId,
  createdByMemberId,
  createdAt: "",
  updatedAt: "",
});

const task = (taskId: string, extra: Partial<Task> = {}): Task =>
  ({ taskId, houseId: "h1", ownerMemberId: "", title: taskId, createdAt: "", updatedAt: "", ...extra }) as Task;

const member = (memberId: string): Member => ({ memberId }) as Member;

describe("isMyProject", () => {
  it("counts the creator as owner", () => {
    expect(isMyProject(project("p1", "me"), "me")).toBe(true);
  });
  it("counts explicit owners and members", () => {
    expect(isMyProject(project("p1", "other"), "me", { ownerIds: ["me"], memberIds: [] })).toBe(true);
    expect(isMyProject(project("p1", "other"), "me", { ownerIds: [], memberIds: ["me"] })).toBe(true);
  });
  it("rejects everyone else", () => {
    expect(isMyProject(project("p1", "other"), "me")).toBe(false);
    expect(isMyProject(project("p1"), "me", { ownerIds: ["x"], memberIds: ["y"] })).toBe(false);
  });
  it("does not match an empty creator against an empty member id", () => {
    expect(isMyProject(project("p1", ""), "")).toBe(false);
  });
});

describe("task/project set helpers", () => {
  const byProject = new Map<string, ProjectTasks>([
    ["p1", { tasks: [task("t1"), task("t2")], hiddenCount: 0 }],
    ["p2", { tasks: [task("t2"), task("t3")], hiddenCount: 1 }],
  ]);

  it("unions task ids across all or selected projects", () => {
    expect([...taskIdsInProjects(byProject)].sort()).toEqual(["t1", "t2", "t3"]);
    expect([...taskIdsInProjects(byProject, ["p1"])].sort()).toEqual(["t1", "t2"]);
    expect([...taskIdsInProjects(byProject, ["missing"])]).toEqual([]);
  });

  it("keeps only tasks in no project", () => {
    const all = [task("t1"), task("t4"), task("t5")];
    expect(tasksInNoProject(all, taskIdsInProjects(byProject)).map((t) => t.taskId)).toEqual(["t4", "t5"]);
  });

  it("finds the projects that hold a task", () => {
    expect([...projectsContainingTask(byProject, "t2")].sort()).toEqual(["p1", "p2"]);
    expect([...projectsContainingTask(byProject, "t9")]).toEqual([]);
  });

  it("counts open tasks only", () => {
    expect(openTaskCount([
      task("a"),
      task("b", { status: "in_progress" }),
      task("c", { status: "done" }),
      task("d", { status: "cancelled" }),
      task("e", { deletedAt: "2026-01-01T00:00:00Z" }),
    ])).toBe(2);
  });
});

describe("projects filter", () => {
  it("defaults to mine only when the viewer has projects", () => {
    expect(effectiveProjectsFilter(undefined, 2)).toBe("mine");
    expect(effectiveProjectsFilter(undefined, 0)).toBe("all");
    expect(effectiveProjectsFilter("all", 2)).toBe("all");
    expect(effectiveProjectsFilter("mine", 0)).toBe("mine");
  });

  it("round-trips per member and ignores junk", () => {
    localStorage.clear();
    expect(readProjectsFilter("m1")).toBeUndefined();
    writeProjectsFilter("m1", "all");
    expect(readProjectsFilter("m1")).toBe("all");
    expect(readProjectsFilter("m2")).toBeUndefined();
    localStorage.setItem("longhouse.projects.filter.m2", "bogus");
    expect(readProjectsFilter("m2")).toBeUndefined();
  });

  it("survives storage that throws", () => {
    const get = vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => { throw new Error("denied"); });
    const set = vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("denied"); });
    try {
      expect(readProjectsFilter("m1")).toBeUndefined();
      expect(() => writeProjectsFilter("m1", "mine")).not.toThrow();
    } finally {
      get.mockRestore();
      set.mockRestore();
    }
  });
});

describe("loaders", () => {
  it("loadMyProjectIds combines creator + grants and tolerates failures", async () => {
    const ps = [project("p1", "me"), project("p2"), project("p3"), project("p4"), project("p5")];
    const calls: string[] = [];
    const client = {
      listProjectMembers: async (id: string) => {
        calls.push(id);
        if (id === "p4") throw new Error("boom");
        return id === "p2" ? [member("me")] : [member("x")];
      },
      listProjectOwners: async (id: string) => (id === "p3" ? [member("me")] : []),
    };
    const mine = await loadMyProjectIds(client, ps, "me");
    expect([...mine].sort()).toEqual(["p1", "p2", "p3"]);
    // The creator match needs no grant lookup.
    expect(calls).not.toContain("p1");
  });

  it("loadProjectTasks keys by project and skips failing projects", async () => {
    const seen: ProjectScopedListRequest[] = [];
    const client = {
      listProjectTasks: async (req: ProjectScopedListRequest) => {
        seen.push(req);
        if (req.projectId === "p2") throw new Error("forbidden");
        return { tasks: [task(`${req.projectId}-t`)], hiddenCount: 2 };
      },
    };
    const out = await loadProjectTasks(client, "h1", [project("p1"), project("p2")]);
    expect([...out.keys()]).toEqual(["p1"]);
    expect(out.get("p1")).toEqual({ tasks: [task("p1-t")], hiddenCount: 2 });
    expect(seen.every((r) => r.houseId === "h1" && r.limit === 500)).toBe(true);
  });
});
