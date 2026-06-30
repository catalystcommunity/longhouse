/**
 * One instance of each generated CSIL service client, all sharing the
 * single CBOR carrier. Pages import these directly — there's no longer
 * a hand-written Repo facade in front of them. Derivation helpers (initials,
 * swatch, groupLabel, tone) live in ~/lib/derive.ts so the clients stay
 * 1:1 with the api shapes.
 *
 * These are the async clients from the generated @longhouse/client package
 * (csilgen `client_style: both` emits a sync and an async surface; the SPA
 * uses the async one over the fetch-based CSIL-RPC carrier).
 */

import {
  AuditAsyncClient,
  AuthAsyncClient,
  BugAsyncClient,
  CommentAsyncClient,
  DependencyAsyncClient,
  DevAuthAsyncClient,
  EventAsyncClient,
  GroupAsyncClient,
  HouseAsyncClient,
  MemberAuditAsyncClient,
  MemberAsyncClient,
  NotificationAsyncClient,
  ProjectAsyncClient,
  RoleAsyncClient,
  SettingsAsyncClient,
  ShareAsyncClient,
  SkillAsyncClient,
  TaskAsyncClient,
  TrashAsyncClient,
  TrustedDomainAsyncClient,
} from "@longhouse/client";
import { cborTransport } from "~/transport/csilrpc";

export const authClient = new AuthAsyncClient(cborTransport);
export const devAuthClient = new DevAuthAsyncClient(cborTransport);
export const memberClient = new MemberAsyncClient(cborTransport);
export const taskClient = new TaskAsyncClient(cborTransport);
export const eventClient = new EventAsyncClient(cborTransport);
export const projectClient = new ProjectAsyncClient(cborTransport);
export const dependencyClient = new DependencyAsyncClient(cborTransport);
export const houseClient = new HouseAsyncClient(cborTransport);
export const roleClient = new RoleAsyncClient(cborTransport);
export const skillClient = new SkillAsyncClient(cborTransport);
export const groupClient = new GroupAsyncClient(cborTransport);
export const commentClient = new CommentAsyncClient(cborTransport);
export const shareClient = new ShareAsyncClient(cborTransport);
export const trustedDomainClient = new TrustedDomainAsyncClient(cborTransport);
export const memberAuditClient = new MemberAuditAsyncClient(cborTransport);
export const settingsClient = new SettingsAsyncClient(cborTransport);
export const bugClient = new BugAsyncClient(cborTransport);
export const notificationClient = new NotificationAsyncClient(cborTransport);
export const auditClient = new AuditAsyncClient(cborTransport);
export const trashClient = new TrashAsyncClient(cborTransport);
