import { For, Show, createMemo, createResource, createSignal } from "solid-js";
import { commentClient } from "~/data/clients";
import { currentMemberId, hasRole } from "~/stores/auth";
import { displayName, initial, memberSwatch } from "~/lib/derive";
import type { Comment, Member } from "@longhouse/client";

/**
 * Discussion thread for a single target (a task or a project). Lists existing
 * comments oldest-first with author + local-time stamp, then a text entry bar
 * with an "Add" button that posts a new comment.
 *
 * Permission gating mirrors the API (which is authoritative):
 *   - delete icon shows for the comment's author OR any house admin.
 *   - edit shows only for the author — admins can remove but not rewrite
 *     someone else's words.
 *
 * All DB timestamps are UTC; we render them in the viewer's local timezone via
 * Date#toLocaleString.
 */

interface Props {
  targetType: "task" | "project";
  targetId: string;
  /** The target's house; used to stamp the new comment. */
  houseId: string;
  /** House members, for resolving author names + avatars. */
  members: Member[];
}

const localStamp = (iso: string): string => {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
};

export const CommentsSection = (props: Props) => {
  const [comments, { refetch, mutate }] = createResource(
    () => ({ t: props.targetType, id: props.targetId }),
    async ({ t, id }) =>
      id ? await commentClient.listComments({ targetType: t as any, targetId: id }) : [],
  );

  const memberById = createMemo(() => {
    const m = new Map<string, Member>();
    for (const x of props.members) m.set(x.memberId, x);
    return m;
  });

  const [draft, setDraft] = createSignal("");
  const [busy, setBusy] = createSignal(false);
  const [err, setErr] = createSignal<string | null>(null);

  const isAdmin = () => hasRole("admin");
  const canDelete = (c: Comment) => c.memberId === currentMemberId() || isAdmin();
  const canEdit = (c: Comment) => c.memberId === currentMemberId();

  const submit = async (e: SubmitEvent) => {
    e.preventDefault();
    const text = draft().trim();
    if (!text || busy()) return;
    setBusy(true);
    setErr(null);
    try {
      await commentClient.createComment({
        houseId: props.houseId,
        targetType: props.targetType as any,
        targetId: props.targetId,
        body: text,
      } as any);
      setDraft("");
      await refetch();
    } catch (e2) {
      setErr(e2 instanceof Error ? e2.message : String(e2));
    } finally {
      setBusy(false);
    }
  };

  const remove = async (c: Comment) => {
    if (!confirm("Delete this comment?")) return;
    mutate((prev) => (prev ?? []).filter((x) => x.commentId !== c.commentId));
    try {
      await commentClient.deleteComment(c.commentId);
      await refetch();
    } catch (e2) {
      setErr(e2 instanceof Error ? e2.message : String(e2));
      await refetch();
    }
  };

  return (
    <div style="margin-top:12px;border-top:1px solid var(--line);padding-top:10px;display:flex;flex-direction:column;gap:10px">
      <span style="font-size:11px;color:var(--ink-mute);text-transform:uppercase;letter-spacing:.04em">
        Discussion
      </span>

      <Show
        when={!comments.loading}
        fallback={<span style="font-size:12px;color:var(--ink-faint)">Loading…</span>}
      >
        <Show
          when={(comments() ?? []).length > 0}
          fallback={<span style="font-size:12.5px;color:var(--ink-faint)">No comments yet.</span>}
        >
          <div style="display:flex;flex-direction:column;gap:10px">
            <For each={comments()}>
              {(c) => (
                <CommentItem
                  comment={c}
                  author={memberById().get(c.memberId)}
                  canEdit={canEdit(c)}
                  canDelete={canDelete(c)}
                  onChanged={async () => { await refetch(); }}
                  onDelete={() => remove(c)}
                  onError={setErr}
                />
              )}
            </For>
          </div>
        </Show>
      </Show>

      <form onSubmit={submit} style="display:flex;gap:8px;align-items:flex-start">
        <textarea
          value={draft()}
          onInput={(e) => setDraft(e.currentTarget.value)}
          placeholder="Add a comment…"
          rows={1}
          onKeyDown={(e) => {
            if ((e.metaKey || e.ctrlKey) && e.key === "Enter") submit(e as any);
          }}
          style="flex:1;padding:7px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px;font-family:inherit;resize:vertical;min-height:34px"
        />
        <button
          type="submit"
          class="btn btn-primary"
          disabled={busy() || !draft().trim()}
          style="padding:7px 14px;white-space:nowrap"
        >
          {busy() ? "…" : "Add"}
        </button>
      </form>

      <Show when={err()}>{(m) => <span style="color:var(--rust);font-size:12px">{m()}</span>}</Show>
    </div>
  );
};

// ─── A single comment row, with inline edit ──────────────────────────────

const CommentItem = (props: {
  comment: Comment;
  author?: Member;
  canEdit: boolean;
  canDelete: boolean;
  onChanged: () => Promise<unknown>;
  onDelete: () => void;
  onError: (m: string) => void;
}) => {
  const [editing, setEditing] = createSignal(false);
  const [body, setBody] = createSignal(props.comment.body);
  const [busy, setBusy] = createSignal(false);

  const author = () =>
    props.author ? displayName(props.author) : "Member";
  const swatch = () => memberSwatch(props.comment.memberId);
  const edited = () =>
    props.comment.updatedAt && props.comment.updatedAt !== props.comment.createdAt;

  const save = async () => {
    const text = body().trim();
    if (!text || busy()) return;
    setBusy(true);
    try {
      await commentClient.updateComment({ ...props.comment, body: text } as any);
      setEditing(false);
      await props.onChanged();
    } catch (e) {
      props.onError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div style="display:flex;gap:8px;align-items:flex-start">
      <span
        class={`a sm ${swatch()}`}
        style="width:22px;height:22px;font-size:11px;flex:none;margin-top:1px"
      >
        {props.author ? initial(props.author) : "?"}
      </span>
      <div style="flex:1;min-width:0">
        <div style="display:flex;align-items:baseline;gap:8px">
          <span style="font-size:12.5px;font-weight:600;color:var(--ink)">{author()}</span>
          <span style="font-size:11px;color:var(--ink-faint)">
            {localStamp(props.comment.createdAt)}
            <Show when={edited()}> · edited</Show>
          </span>
          <span style="margin-left:auto;display:flex;gap:4px">
            <Show when={props.canEdit && !editing()}>
              <button
                type="button"
                aria-label="Edit comment"
                onClick={() => setEditing(true)}
                style="background:transparent;border:0;color:var(--ink-mute);font-size:13px;cursor:pointer;padding:0 4px"
              >
                ✎
              </button>
            </Show>
            <Show when={props.canDelete}>
              <button
                type="button"
                aria-label="Delete comment"
                onClick={() => props.onDelete()}
                style="background:transparent;border:0;color:var(--ink-mute);font-size:16px;line-height:1;cursor:pointer;padding:0 4px"
              >
                ×
              </button>
            </Show>
          </span>
        </div>
        <Show
          when={editing()}
          fallback={
            <p style="margin:2px 0 0;font-size:13px;color:var(--ink);white-space:pre-wrap;word-break:break-word">
              {props.comment.body}
            </p>
          }
        >
          <div style="display:flex;flex-direction:column;gap:6px;margin-top:4px">
            <textarea
              value={body()}
              onInput={(e) => setBody(e.currentTarget.value)}
              rows={2}
              style="padding:6px 9px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px;font-family:inherit;resize:vertical"
            />
            <div style="display:flex;gap:6px;justify-content:flex-end">
              <button
                type="button"
                class="btn btn-ghost"
                onClick={() => { setBody(props.comment.body); setEditing(false); }}
                disabled={busy()}
                style="padding:4px 10px"
              >
                Cancel
              </button>
              <button
                type="button"
                class="btn btn-primary"
                onClick={save}
                disabled={busy() || !body().trim()}
                style="padding:4px 12px"
              >
                {busy() ? "Saving…" : "Save"}
              </button>
            </div>
          </div>
        </Show>
      </div>
    </div>
  );
};
