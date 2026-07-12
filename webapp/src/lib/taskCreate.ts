import type { Task } from "@longhouse/client";

type TaskCreatePayload = Omit<Task, "taskId" | "createdAt" | "updatedAt"> &
  Partial<Pick<Task, "taskId" | "createdAt" | "updatedAt">>;

const RECEIVE_ONLY_PLACEHOLDER = "";

export function normalizeTaskCreate(input: TaskCreatePayload): Task {
  return {
    ...input,
    taskId: input.taskId ?? RECEIVE_ONLY_PLACEHOLDER,
    createdAt: input.createdAt ?? RECEIVE_ONLY_PLACEHOLDER,
    updatedAt: input.updatedAt ?? RECEIVE_ONLY_PLACEHOLDER,
  };
}
