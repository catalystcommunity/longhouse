import { For, Show, createMemo, createResource, createSignal } from "solid-js";
import { AuthGate } from "~/components/AuthGate";
import { groupClient, memberClient, skillClient } from "~/data/clients";
import { displayName, initial, memberSwatch } from "~/lib/derive";
import { hasRole, useCurrentHouseId } from "~/stores/auth";
import type { Group, Member, Skill } from "~/api/types.gen";

export const GroupsPage = () => {
  const houseId = useCurrentHouseId();
  const isAdmin = () => hasRole("admin");

  const [groups, { refetch }] = createResource(
    () => houseId(),
    async (h) => groupClient.listGroups({ houseId: h }),
  );
  const [members] = createResource(
    () => houseId(),
    async (h) => memberClient.listMembers({ houseId: h }),
  );
  const [skills] = createResource(
    () => houseId(),
    async (h) => skillClient.listSkills({ houseId: h }),
  );

  const [composerOpen, setComposerOpen] = createSignal(false);

  return (
    <AuthGate>
      <div class="section-hd reveal">
        <h2>Groups <em>buckets of members</em></h2>
        <p class="lead">Admin-curated subsets — "House team", "Garden crew", whatever serves the work.</p>
        <Show when={isAdmin()}>
          <div style="margin-top:12px">
            <button class="btn btn-primary" onClick={() => setComposerOpen((v) => !v)}>
              {composerOpen() ? "Cancel" : "New group"}
            </button>
          </div>
        </Show>
      </div>

      <Show when={composerOpen()}>
        <GroupForm
          houseId={houseId()!}
          onCancel={() => setComposerOpen(false)}
          onSaved={async () => { setComposerOpen(false); await refetch(); }}
        />
      </Show>

      <Show
        when={!groups.loading}
        fallback={<p style="padding:24px;color:var(--ink-mute)">Loading…</p>}
      >
        <Show
          when={(groups() ?? []).length > 0}
          fallback={
            <section style="margin-top:24px;padding:32px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);text-align:center;color:var(--ink-mute)">
              <p style="margin:0 0 4px"><b>No groups yet.</b></p>
              <p style="margin:0">Create a group to bundle members for events, tasks, or skills.</p>
            </section>
          }
        >
          <div style="display:grid;grid-template-columns:1fr;gap:16px;margin-top:8px">
            <For each={groups()!}>
              {(g) => (
                <GroupCard
                  group={g}
                  houseMembers={members() ?? []}
                  houseSkills={skills() ?? []}
                  canAdmin={isAdmin()}
                  onChanged={async () => { await refetch(); }}
                />
              )}
            </For>
          </div>
        </Show>
      </Show>
    </AuthGate>
  );
};

// ─── Group card ────────────────────────────────────────────────────────

const GroupCard = (props: {
  group: Group;
  houseMembers: Member[];
  houseSkills: Skill[];
  canAdmin: boolean;
  onChanged: () => Promise<unknown>;
}) => {
  const [members, { refetch: refetchMembers }] = createResource(
    () => props.group.groupId,
    async (gid) => groupClient.listGroupMembers({ houseId: "", memberId: gid }),
  );
  const [skills, { refetch: refetchSkills }] = createResource(
    () => props.group.groupId,
    async (gid) => skillClient.listGroupSkills(gid),
  );

  const [editing, setEditing] = createSignal(false);

  const remove = async () => {
    if (!confirm(`Delete group "${props.group.name}"?`)) return;
    try {
      await groupClient.deleteGroup(props.group.groupId);
      await props.onChanged();
    } catch (e) { alert(e instanceof Error ? e.message : String(e)); }
  };

  return (
    <article style="padding:18px 22px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);box-shadow:var(--shadow-low)">
      <Show
        when={!editing()}
        fallback={
          <GroupForm
            houseId={props.group.houseId}
            existing={props.group}
            onCancel={() => setEditing(false)}
            onSaved={async () => { setEditing(false); await props.onChanged(); }}
          />
        }
      >
        <div style="display:flex;justify-content:space-between;align-items:start;gap:12px">
          <div>
            <h3 style="margin:0 0 4px;font-family:var(--display);font-size:22px;color:var(--grass-4)">
              {props.group.name}
            </h3>
            <Show when={props.group.description}>
              {(d) => <p style="margin:0;color:var(--ink-mute);font-size:14px">{d()}</p>}
            </Show>
          </div>
          <Show when={props.canAdmin}>
            <div style="display:flex;gap:8px">
              <button class="btn-quiet" onClick={() => setEditing(true)} style="font-size:12px;padding:4px 10px">Edit</button>
              <button class="btn-quiet" onClick={remove} style="font-size:12px;padding:4px 10px;color:var(--rust)">Delete</button>
            </div>
          </Show>
        </div>
      </Show>

      <div style="margin-top:14px;display:grid;grid-template-columns:80px 1fr;gap:6px 12px;align-items:start;font-size:13px">
        <span style="color:var(--ink-mute)">Members</span>
        <MemberPickerLite
          assigned={members() ?? []}
          all={props.houseMembers}
          canEdit={props.canAdmin}
          onAdd={async (memberId) => {
            await groupClient.addGroupMember({ groupId: props.group.groupId, memberId });
            await refetchMembers();
          }}
          onRemove={async (memberId) => {
            await groupClient.removeGroupMember({ groupId: props.group.groupId, memberId });
            await refetchMembers();
          }}
        />

        <span style="color:var(--ink-mute)">Skills</span>
        <SkillPickerLite
          assigned={skills() ?? []}
          all={props.houseSkills}
          canEdit={props.canAdmin}
          onAdd={async (skillId) => {
            await skillClient.addGroupSkill({ groupId: props.group.groupId, skillId });
            await refetchSkills();
          }}
          onRemove={async (skillId) => {
            await skillClient.removeGroupSkill({ groupId: props.group.groupId, skillId });
            await refetchSkills();
          }}
        />
      </div>
    </article>
  );
};

// ─── Composer ──────────────────────────────────────────────────────────

const GroupForm = (props: {
  houseId: string;
  existing?: Group;
  onCancel: () => void;
  onSaved: () => Promise<void> | void;
}) => {
  const [name, setName] = createSignal(props.existing?.name ?? "");
  const [description, setDesc] = createSignal(props.existing?.description ?? "");
  const [busy, setBusy] = createSignal(false);
  const [err, setErr] = createSignal<string | null>(null);

  const submit = async (e: SubmitEvent) => {
    e.preventDefault();
    if (!name().trim() || busy()) return;
    setBusy(true);
    setErr(null);
    try {
      if (props.existing) {
        await groupClient.updateGroup({
          ...props.existing,
          name: name().trim(),
          description: description().trim() || undefined,
        } as any);
      } else {
        await groupClient.createGroup({
          groupId: "",
          houseId: props.houseId,
          name: name().trim(),
          description: description().trim() || undefined,
          createdAt: "",
          updatedAt: "",
        } as any);
      }
      await props.onSaved();
    } catch (e2) { setErr(e2 instanceof Error ? e2.message : String(e2)); }
    finally { setBusy(false); }
  };

  return (
    <form
      onSubmit={submit}
      style="margin:8px 0 12px;padding:14px 16px;background:color-mix(in oklab, var(--paper) 92%, var(--ocean-1) 8%);border:1px solid var(--line);border-radius:var(--r-md);display:grid;grid-template-columns:1fr 2fr auto auto;gap:10px;align-items:end"
    >
      <label style="display:flex;flex-direction:column;gap:4px">
        <span style="font-size:11px;color:var(--ink-mute)">Name</span>
        <input
          type="text" value={name()} onInput={(e) => setName(e.currentTarget.value)}
          required autofocus
          style="padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px"
        />
      </label>
      <label style="display:flex;flex-direction:column;gap:4px">
        <span style="font-size:11px;color:var(--ink-mute)">Description</span>
        <input
          type="text" value={description()} onInput={(e) => setDesc(e.currentTarget.value)}
          style="padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px"
        />
      </label>
      <button class="btn btn-primary" disabled={busy() || !name().trim()} type="submit">
        {busy() ? "…" : props.existing ? "Save" : "Create"}
      </button>
      <button type="button" class="btn btn-ghost" onClick={props.onCancel} disabled={busy()}>Cancel</button>
      <Show when={err()}>{(m) => <span style="color:var(--rust);grid-column:1/-1;margin:0">{m()}</span>}</Show>
    </form>
  );
};

// ─── Pickers ───────────────────────────────────────────────────────────

const MemberPickerLite = (props: {
  assigned: Member[];
  all: Member[];
  canEdit: boolean;
  onAdd: (memberId: string) => Promise<unknown>;
  onRemove: (memberId: string) => Promise<unknown>;
}) => {
  const [selected, setSelected] = createSignal("");
  const [busy, setBusy] = createSignal(false);
  const available = createMemo(() => {
    const taken = new Set(props.assigned.map((m) => m.memberId));
    return props.all.filter((m) => !taken.has(m.memberId));
  });
  return (
    <div style="display:flex;flex-wrap:wrap;gap:6px;align-items:center">
      <Show
        when={props.assigned.length > 0}
        fallback={<span style="font-size:12px;color:var(--ink-mute);font-style:italic">none yet</span>}
      >
        <For each={props.assigned}>
          {(m) => (
            <span style="display:inline-flex;align-items:center;gap:4px;padding:3px 8px 3px 3px;background:color-mix(in oklab, var(--paper) 90%, var(--ocean-1) 10%);border:1px solid var(--line);border-radius:9999px">
              <span class={`a sm ${memberSwatch(m.memberId)}`} style="width:18px;height:18px;font-size:10px">{initial(m)}</span>
              <span style="font-size:12px">{displayName(m)}</span>
              <Show when={props.canEdit}>
                <button
                  type="button" onClick={() => { setBusy(true); props.onRemove(m.memberId).finally(() => setBusy(false)); }}
                  style="background:transparent;border:0;cursor:pointer;color:var(--ink-mute);font-size:13px;line-height:1;padding:0"
                  disabled={busy()}
                  title="Remove"
                >×</button>
              </Show>
            </span>
          )}
        </For>
      </Show>
      <Show when={props.canEdit && available().length > 0}>
        <select
          value={selected()}
          onChange={(e) => setSelected(e.currentTarget.value)}
          style="padding:3px 8px;border:1px dashed var(--line);border-radius:9999px;background:var(--paper);font-size:12px"
          disabled={busy()}
        >
          <option value="">+ add member</option>
          <For each={available()}>{(m) => <option value={m.memberId}>{displayName(m)}</option>}</For>
        </select>
        <Show when={selected()}>
          <button
            type="button" class="btn-quiet"
            disabled={busy()}
            onClick={async () => {
              setBusy(true);
              try { await props.onAdd(selected()); setSelected(""); }
              catch (e) { alert(e instanceof Error ? e.message : String(e)); }
              finally { setBusy(false); }
            }}
            style="font-size:11px;padding:2px 8px"
          >add</button>
        </Show>
      </Show>
    </div>
  );
};

const SkillPickerLite = (props: {
  assigned: Skill[];
  all: Skill[];
  canEdit: boolean;
  onAdd: (skillId: string) => Promise<unknown>;
  onRemove: (skillId: string) => Promise<unknown>;
}) => {
  const [selected, setSelected] = createSignal("");
  const [busy, setBusy] = createSignal(false);
  const available = createMemo(() => {
    const taken = new Set(props.assigned.map((s) => s.skillId));
    return props.all.filter((s) => !taken.has(s.skillId));
  });
  return (
    <div style="display:flex;flex-wrap:wrap;gap:6px;align-items:center">
      <Show
        when={props.assigned.length > 0}
        fallback={<span style="font-size:12px;color:var(--ink-mute);font-style:italic">none yet</span>}
      >
        <For each={props.assigned}>
          {(s) => (
            <span style="display:inline-flex;align-items:center;gap:4px;padding:2px 8px;font-size:12px;border:1px solid var(--grass-4);background:color-mix(in oklab, var(--paper) 88%, var(--grass-4) 12%);border-radius:9999px;color:var(--ink)">
              {s.name}
              <Show when={props.canEdit}>
                <button
                  type="button" onClick={() => { setBusy(true); props.onRemove(s.skillId).finally(() => setBusy(false)); }}
                  style="background:transparent;border:0;cursor:pointer;color:var(--ink-mute);font-size:13px;line-height:1;padding:0"
                  disabled={busy()} title="Remove"
                >×</button>
              </Show>
            </span>
          )}
        </For>
      </Show>
      <Show when={props.canEdit && available().length > 0}>
        <select
          value={selected()}
          onChange={(e) => setSelected(e.currentTarget.value)}
          style="padding:3px 8px;border:1px dashed var(--line);border-radius:9999px;background:var(--paper);font-size:12px"
          disabled={busy()}
        >
          <option value="">+ add skill</option>
          <For each={available()}>{(s) => <option value={s.skillId}>{s.name}</option>}</For>
        </select>
        <Show when={selected()}>
          <button
            type="button" class="btn-quiet"
            disabled={busy()}
            onClick={async () => {
              setBusy(true);
              try { await props.onAdd(selected()); setSelected(""); }
              catch (e) { alert(e instanceof Error ? e.message : String(e)); }
              finally { setBusy(false); }
            }}
            style="font-size:11px;padding:2px 8px"
          >add</button>
        </Show>
      </Show>
    </div>
  );
};
