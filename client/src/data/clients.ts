/**
 * One instance of each generated CSIL service client, all sharing the
 * single CBOR transport. Pages import these directly — there's no longer
 * a hand-written Repo facade in front of them. Derivation helpers (initials,
 * swatch, groupLabel, tone) live in ~/lib/derive.ts so the clients stay
 * 1:1 with the api shapes.
 */

import {
  AuditClient,
  AuthClient,
  BugClient,
  CommentClient,
  DependencyClient,
  DevAuthClient,
  EventClient,
  GroupClient,
  HouseClient,
  MemberAuditClient,
  MemberClient,
  NotificationClient,
  ProjectClient,
  RoleClient,
  SettingsClient,
  ShareClient,
  SkillClient,
  TaskClient,
  TrashClient,
  TrustedDomainClient,
} from "~/api/client.gen";
import { cborTransport } from "~/transport/csilrpc";

export const authClient = new AuthClient(cborTransport);
export const devAuthClient = new DevAuthClient(cborTransport);
export const memberClient = new MemberClient(cborTransport);
export const taskClient = new TaskClient(cborTransport);
export const eventClient = new EventClient(cborTransport);
export const projectClient = new ProjectClient(cborTransport);
export const dependencyClient = new DependencyClient(cborTransport);
export const houseClient = new HouseClient(cborTransport);
export const roleClient = new RoleClient(cborTransport);
export const skillClient = new SkillClient(cborTransport);
export const groupClient = new GroupClient(cborTransport);
export const commentClient = new CommentClient(cborTransport);
export const shareClient = new ShareClient(cborTransport);
export const trustedDomainClient = new TrustedDomainClient(cborTransport);
export const memberAuditClient = new MemberAuditClient(cborTransport);
export const settingsClient = new SettingsClient(cborTransport);
export const bugClient = new BugClient(cborTransport);
export const notificationClient = new NotificationClient(cborTransport);
export const auditClient = new AuditClient(cborTransport);
export const trashClient = new TrashClient(cborTransport);
