/**
 * LiveRepo — hits the api when the user is authenticated AND has a house
 * selected; otherwise transparently falls back to MockRepo. That gives us:
 *   - the static comp's beauty for unauthenticated visitors
 *   - real data once you sign in and a house is picked
 *   - graceful degradation if a single request 5xx's
 *
 * For now only the members surface is wired to the real api — it's the
 * cleanest mapping and exercises the auth + per-house authz path end to
 * end. Tasks/events/projects stay on MockRepo until the api grows
 * CSIL-RPC endpoints (the existing REST shapes don't carry the UI fields
 * the comp uses; mapping them is its own piece of work).
 */

import { jsonFetch } from "~/transport/http";
import { currentToken, useCurrentHouseId } from "~/stores/auth";
import { repo as mockRepo } from "./repo";
import type { Repo } from "./repo";
import type { Member } from "./types";

interface ApiMember {
  member_id: string;
  house_id: string;
  linkkeys_domain: string;
  linkkeys_user_id: string;
  display_name?: string;
  last_seen_at?: string;
}

const swatchFor = (id: string): Member["swatch"] => {
  const variants: Member["swatch"][] = ["a1", "a2", "a3", "a4"];
  let h = 0;
  for (const ch of id) h = (h * 31 + ch.charCodeAt(0)) | 0;
  return variants[Math.abs(h) % variants.length];
};

const toSpaMember = (m: ApiMember): Member => {
  const name = m.display_name ?? `${m.linkkeys_user_id}@${m.linkkeys_domain}`;
  const firstWord = name.split(/\s+/)[0] ?? "?";
  return {
    id: m.member_id,
    name,
    initials: (firstWord.match(/\S/)?.[0] ?? "?").toUpperCase(),
    swatch: swatchFor(m.member_id),
    status: m.last_seen_at ? "active" : "offline",
    lastSeenLabel: undefined,
  };
};

const houseScope = () => ({ token: currentToken(), houseId: useCurrentHouseId()() });

class LiveRepoImpl implements Repo {
  async listMembers(): Promise<Member[]> {
    const { token, houseId } = houseScope();
    if (!token || !houseId) return mockRepo.listMembers();
    try {
      const api = await jsonFetch<ApiMember[]>(`/api/v1/houses/${houseId}/members`);
      return api.map(toSpaMember);
    } catch {
      return mockRepo.listMembers();
    }
  }

  async getMember(id: string) {
    const { token, houseId } = houseScope();
    if (!token || !houseId) return mockRepo.getMember(id);
    try {
      const api = await jsonFetch<ApiMember>(`/api/v1/houses/${houseId}/members/${id}`);
      return toSpaMember(api);
    } catch {
      return mockRepo.getMember(id);
    }
  }

  // Deferred to MockRepo until the api carries the UI-flavored fields the
  // comp uses (tones, organizer colors, task estimates, group buckets, etc.)
  listTasks() {
    return mockRepo.listTasks();
  }
  listEvents() {
    return mockRepo.listEvents();
  }
  listProjects() {
    return mockRepo.listProjects();
  }
  getProject(slug: string) {
    return mockRepo.getProject(slug);
  }
}

export const liveRepo: Repo = new LiveRepoImpl();
