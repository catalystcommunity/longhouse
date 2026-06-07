import { describe, expect, it } from "vitest";
import { filterAddable, nodeKey, splitNodeValue, type Candidate } from "./DependenciesSection";

describe("nodeKey / splitNodeValue", () => {
  it("round-trips a type+id", () => {
    const v = nodeKey("task", "abc-123");
    expect(v).toBe("task:abc-123");
    expect(splitNodeValue(v)).toEqual({ type: "task", id: "abc-123" });
  });

  it("keeps ids that contain a colon intact (splits on the first only)", () => {
    expect(splitNodeValue("project:a:b:c")).toEqual({ type: "project", id: "a:b:c" });
  });
});

describe("filterAddable", () => {
  const candidates: Candidate[] = [
    { type: "task", id: "t1", label: "Task 1" },
    { type: "task", id: "t2", label: "Task 2" },
    { type: "project", id: "p1", label: "Project 1" },
  ];

  it("returns everything when nothing is depended on yet", () => {
    expect(filterAddable(candidates, [])).toHaveLength(3);
  });

  it("hides candidates already in the dependency list", () => {
    const deps = [
      { type: "task", id: "t1" },
      { type: "project", id: "p1" },
    ];
    const got = filterAddable(candidates, deps);
    expect(got.map((c) => c.id)).toEqual(["t2"]);
  });

  it("matches on type AND id (same id, different type stays addable)", () => {
    const deps = [{ type: "project", id: "t1" }]; // project t1 != task t1
    const got = filterAddable(candidates, deps);
    expect(got.map((c) => nodeKey(c.type, c.id))).toContain("task:t1");
  });

  it("coerces an unknown-typed dependency type to string before comparing", () => {
    const deps = [{ type: "task" as unknown, id: "t2" }];
    const got = filterAddable(candidates, deps);
    expect(got.map((c) => c.id)).toEqual(["t1", "p1"]);
  });
});
