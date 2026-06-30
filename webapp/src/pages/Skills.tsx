import { For, Show, createResource, createSignal } from "solid-js";
import { AuthGate } from "~/components/AuthGate";
import { skillClient } from "~/data/clients";
import { hasRole, useCurrentHouseId } from "~/stores/auth";
import type { Skill } from "@longhouse/client";

export const SkillsPage = () => {
  const houseId = useCurrentHouseId();
  const isAdmin = () => hasRole("admin");

  const [skills, { refetch }] = createResource(
    () => houseId(),
    async (h) => skillClient.listSkills({ houseId: h }),
  );

  const [composerOpen, setComposerOpen] = createSignal(false);

  return (
    <AuthGate>
      <div class="section-hd reveal">
        <h2>Skills <em>what people are good at</em></h2>
        <p class="lead">A house-wide catalog. Members add their own; admins can add for anyone.
        Attach to groups on the Groups page; to people on the Members page.</p>
        <Show when={isAdmin()}>
          <div style="margin-top:12px">
            <button class="btn btn-primary" onClick={() => setComposerOpen((v) => !v)}>
              {composerOpen() ? "Cancel" : "New skill"}
            </button>
          </div>
        </Show>
      </div>

      <Show when={composerOpen()}>
        <SkillForm
          houseId={houseId()!}
          onCancel={() => setComposerOpen(false)}
          onSaved={async () => { setComposerOpen(false); await refetch(); }}
        />
      </Show>

      <Show
        when={!skills.loading}
        fallback={<p style="padding:24px;color:var(--ink-mute)">Loading…</p>}
      >
        <Show
          when={(skills() ?? []).length > 0}
          fallback={
            <section style="margin-top:24px;padding:32px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);text-align:center;color:var(--ink-mute)">
              <p style="margin:0 0 4px"><b>No skills yet.</b></p>
              <p style="margin:0">Capabilities like "Plumbing", "Bookkeeping", or "First aid". Add one to begin attaching it to people or groups.</p>
            </section>
          }
        >
          <ul style="margin:8px 0 0;padding:0;list-style:none;display:grid;grid-template-columns:1fr;gap:10px">
            <For each={skills()!}>
              {(s) => (
                <SkillRow
                  skill={s}
                  canAdmin={isAdmin()}
                  onChanged={async () => { await refetch(); }}
                />
              )}
            </For>
          </ul>
        </Show>
      </Show>
    </AuthGate>
  );
};

const SkillRow = (props: { skill: Skill; canAdmin: boolean; onChanged: () => Promise<unknown> }) => {
  const [editing, setEditing] = createSignal(false);
  const remove = async () => {
    if (!confirm(`Delete skill "${props.skill.name}"?`)) return;
    try {
      await skillClient.deleteSkill(props.skill.skillId);
      await props.onChanged();
    } catch (e) { alert(e instanceof Error ? e.message : String(e)); }
  };
  return (
    <li style="padding:14px 18px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-md);display:flex;justify-content:space-between;align-items:center;gap:12px">
      <Show
        when={!editing()}
        fallback={
          <SkillForm
            houseId={props.skill.houseId}
            existing={props.skill}
            onCancel={() => setEditing(false)}
            onSaved={async () => { setEditing(false); await props.onChanged(); }}
          />
        }
      >
        <div>
          <div style="font-weight:600">{props.skill.name}</div>
          <Show when={props.skill.description}>
            {(d) => <div style="color:var(--ink-mute);font-size:13px">{d()}</div>}
          </Show>
        </div>
        <Show when={props.canAdmin}>
          <div style="display:flex;gap:8px">
            <button class="btn-quiet" style="font-size:12px;padding:4px 10px" onClick={() => setEditing(true)}>Edit</button>
            <button class="btn-quiet" style="font-size:12px;padding:4px 10px;color:var(--rust)" onClick={remove}>Delete</button>
          </div>
        </Show>
      </Show>
    </li>
  );
};

const SkillForm = (props: {
  houseId: string;
  existing?: Skill;
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
        await skillClient.updateSkill({
          ...props.existing,
          name: name().trim(),
          description: description().trim() || undefined,
        } as any);
      } else {
        await skillClient.createSkill({
          skillId: "",
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
      style="display:grid;grid-template-columns:1fr 2fr auto auto;gap:10px;align-items:end;width:100%"
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
