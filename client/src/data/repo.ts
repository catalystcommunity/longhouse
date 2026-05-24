import type { Event, Member, Project, Task } from "./types";
import { events, members, projects, tasks, memberById, projectBySlug } from "./mocks";

/**
 * Repository interface. Today this returns mocked data synchronously
 * for speed of iteration. The CBOR transport will replace these
 * implementations with async fetches against the Longhouse API.
 */
export interface Repo {
  listTasks(): Promise<Task[]>;
  listEvents(): Promise<Event[]>;
  listMembers(): Promise<Member[]>;
  getMember(id: string): Promise<Member | undefined>;
  listProjects(): Promise<Project[]>;
  getProject(slug: string): Promise<Project | undefined>;
}

class MockRepo implements Repo {
  async listTasks()    { return tasks; }
  async listEvents()   { return events; }
  async listMembers()  { return members; }
  async getMember(id: string)    { return memberById.get(id); }
  async listProjects() { return projects; }
  async getProject(slug: string) { return projectBySlug.get(slug); }
}

export const repo: Repo = new MockRepo();
