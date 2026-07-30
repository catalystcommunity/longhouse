import type { Comment, Event, Project, Task } from "@longhouse/client";

type TaskCreatePayload = Omit<Task, "taskId" | "createdAt" | "updatedAt"> &
  Partial<Pick<Task, "taskId" | "createdAt" | "updatedAt">>;
type ProjectCreatePayload = Omit<Project, "projectId" | "createdAt" | "updatedAt"> &
  Partial<Pick<Project, "projectId" | "createdAt" | "updatedAt">>;
type EventCreatePayload = Omit<Event, "eventId" | "ownerMemberId" | "createdAt" | "updatedAt"> &
  Partial<Pick<Event, "eventId" | "ownerMemberId" | "createdAt" | "updatedAt">>;
type CommentCreatePayload = Omit<Comment, "commentId" | "memberId" | "createdAt" | "updatedAt"> &
  Partial<Pick<Comment, "commentId" | "memberId" | "createdAt" | "updatedAt">>;

const RECEIVE_ONLY_PLACEHOLDER = "";

export function normalizeTaskCreate(input: TaskCreatePayload): Task {
  return {
    ...input,
    taskId: input.taskId ?? RECEIVE_ONLY_PLACEHOLDER,
    createdAt: input.createdAt ?? RECEIVE_ONLY_PLACEHOLDER,
    updatedAt: input.updatedAt ?? RECEIVE_ONLY_PLACEHOLDER,
  };
}

export function normalizeProjectCreate(input: ProjectCreatePayload): Project {
  return {
    ...input,
    projectId: input.projectId ?? RECEIVE_ONLY_PLACEHOLDER,
    createdAt: input.createdAt ?? RECEIVE_ONLY_PLACEHOLDER,
    updatedAt: input.updatedAt ?? RECEIVE_ONLY_PLACEHOLDER,
  };
}

export function normalizeEventCreate(input: EventCreatePayload): Event {
  return {
    ...input,
    eventId: input.eventId ?? RECEIVE_ONLY_PLACEHOLDER,
    ownerMemberId: input.ownerMemberId ?? RECEIVE_ONLY_PLACEHOLDER,
    createdAt: input.createdAt ?? RECEIVE_ONLY_PLACEHOLDER,
    updatedAt: input.updatedAt ?? RECEIVE_ONLY_PLACEHOLDER,
  };
}

export function normalizeCommentCreate(input: CommentCreatePayload): Comment {
  return {
    ...input,
    commentId: input.commentId ?? RECEIVE_ONLY_PLACEHOLDER,
    memberId: input.memberId ?? RECEIVE_ONLY_PLACEHOLDER,
    createdAt: input.createdAt ?? RECEIVE_ONLY_PLACEHOLDER,
    updatedAt: input.updatedAt ?? RECEIVE_ONLY_PLACEHOLDER,
  };
}
