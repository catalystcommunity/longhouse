import { For, Show, createMemo, createResource, createSignal } from "solid-js";
import { A } from "@solidjs/router";
import { AuthGate } from "~/components/AuthGate";
import { memberClient, roleClient, skillClient } from "~/data/clients";
import { displayName, initial, lastSeenLabel, memberStatus, memberSwatch } from "~/lib/derive";
import { currentMemberId, hasRole, useCurrentHouseId } from "~/stores/auth";
import { loadHouses } from "~/lib/session";
import type { Member, Role, Skill } from "~/api/types.gen";

export const MembersPage = () => {
  const houseId = useCurrentHouseId();
  const isAdmin = () => hasRole("admin");
  const [members, { refetch }] = createResource(
    () => houseId(),
    async (h) => memberClient.listMembers({ houseId: h }),
  );
  const [roles, { refetch: refetchRoles }] = createResource(
    () => houseId(),
    async (h) => roleClient.listRoles({ houseId: h }),
  );
  const [skills, { refetch: refetchSkills }] = createResource(
    () => houseId(),
    async (h) => skillClient.listSkills({ houseId: h }),
  );

  const [inviteOpen, setInviteOpen] = createSignal(false);

  return (
    <AuthGate>
      <div class="section-hd reveal">
        <h2>Members <em>the household</em></h2>
        <p class="lead">Everyone with access to this Longhouse instance.</p>
        <Show when={isAdmin()}>
          <div style="margin-top:12px">
            <button class="btn btn-primary" onClick={() => setInviteOpen((v) => !v)}>
              {inviteOpen() ? "Cancel" : "Invite member"}
            </button>
          </div>
        </Show>
      </div>

      <Show when={inviteOpen()}>
        <InviteForm
          houseId={houseId()!}
          onCancel={() => setInviteOpen(false)}
          onInvited={async () => { setInviteOpen(false); await refetch(); }}
        />
      </Show>

      <section class="card folks reveal d1" style="margin-top:0;padding: 8px 22px 18px">
        <Show
          when={!members.loading}
          fallback={<p style="padding:24px;color:var(--ink-mute)">Loading…</p>}
        >
          <Show
            when={(members() ?? []).length > 0}
            fallback={<p style="padding:24px;color:var(--ink-mute)">No members in this house yet.</p>}
          >
            <For each={members()!}>
              {(m) => (
                <MemberRow
                  member={m}
                  houseId={houseId()!}
                  houseRoles={roles() ?? []}
                  houseSkills={skills() ?? []}
                  onChanged={async () => {
                    await refetch();
                    await refetchRoles();
                    await refetchSkills();
                  }}
                />
              )}
            </For>
          </Show>
        </Show>
      </section>
    </AuthGate>
  );
};

// ─── Invite ───────────────────────────────────────────────────────────

const InviteForm = (props: {
  houseId: string;
  onCancel: () => void;
  onInvited: () => Promise<void> | void;
}) => {
  const [domain, setDomain] = createSignal("");
  const [userId, setUserId] = createSignal("");
  const [displayN, setDisplayN] = createSignal("");
  const [busy, setBusy] = createSignal(false);
  const [err, setErr] = createSignal<string | null>(null);

  const submit = async (e: SubmitEvent) => {
    e.preventDefault();
    if (!domain().trim() || !userId().trim() || busy()) return;
    setBusy(true);
    setErr(null);
    try {
      await memberClient.createMember({
        memberId: "",
        houseId: props.houseId,
        linkkeysDomain: domain().trim(),
        linkkeysUserId: userId().trim(),
        displayName: displayN().trim() || undefined,
        createdAt: "",
        updatedAt: "",
      } as any);
      await props.onInvited();
    } catch (e2) {
      setErr(e2 instanceof Error ? e2.message : String(e2));
    } finally {
      setBusy(false);
    }
  };

  return (
    <form
      onSubmit={submit}
      style="margin-bottom:16px;padding:16px 20px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);display:grid;grid-template-columns:1fr 1fr 1fr auto auto;gap:10px;align-items:end"
    >
      <label style="display:flex;flex-direction:column;gap:4px">
        <span style="font-size:12px;color:var(--ink-mute)">Linkkeys domain</span>
        <input
          type="text"
          value={domain()}
          onInput={(e) => setDomain(e.currentTarget.value)}
          placeholder="todandlorna.com"
          required autofocus
          style="padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px"
        />
      </label>
      <label style="display:flex;flex-direction:column;gap:4px">
        <span style="font-size:12px;color:var(--ink-mute)">Linkkeys user_id (UUID)</span>
        <input
          type="text"
          value={userId()}
          onInput={(e) => setUserId(e.currentTarget.value)}
          placeholder="019d6ac9-…"
          required
          style="padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px;font-family:var(--mono, monospace);font-size:12px"
        />
        <span style="font-size:11px;color:var(--ink-mute)">
          The person's linkkeys account id — not their email. Ask them for it,
          or copy it from their linkkeys profile.
        </span>
      </label>
      <label style="display:flex;flex-direction:column;gap:4px">
        <span style="font-size:12px;color:var(--ink-mute)">Display name (optional)</span>
        <input
          type="text"
          value={displayN()}
          onInput={(e) => setDisplayN(e.currentTarget.value)}
          placeholder="how the UI shows them"
          style="padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px"
        />
      </label>
      <button class="btn btn-primary" disabled={busy() || !domain().trim() || !userId().trim()} type="submit">
        {busy() ? "Saving…" : "Invite"}
      </button>
      <button type="button" class="btn btn-ghost" onClick={props.onCancel} disabled={busy()}>Cancel</button>
      <Show when={!domain().trim() || !userId().trim()}>
        <p style="color:var(--ink-mute);font-size:12px;grid-column:1/-1;margin:0">
          Fill in both the linkkeys domain and user_id to enable Invite.
        </p>
      </Show>
      <p style="color:var(--ink-mute);font-size:12px;grid-column:1/-1;margin:0">
        Adding everyone from a domain? Skip per-person invites — add a{" "}
        <A href="/settings" style="color:var(--ocean-2)">trusted domain</A>{" "}
        and anyone signing in from it auto-joins on their next sign-in.
      </p>
      <Show when={err()}>
        {(m) => <p style="color:var(--rust);font-size:13px;grid-column:1/-1;margin:0">{m()}</p>}
      </Show>
    </form>
  );
};

// ─── Row ──────────────────────────────────────────────────────────────

const MemberRow = (props: {
  member: Member;
  houseId: string;
  houseRoles: Role[];
  houseSkills: Skill[];
  onChanged: () => Promise<unknown>;
}) => {
  const isSelf = () => currentMemberId() === props.member.memberId;
  const canEdit = () => isSelf() || hasRole("admin");
  const canAdmin = () => hasRole("admin");
  const [editing, setEditing] = createSignal(false);
  const status = () => memberStatus(props.member);

  const [memberRoles, { refetch: refetchRoles }] = createResource(
    () => ({ houseId: props.houseId, memberId: props.member.memberId }),
    async (req) => roleClient.listMemberRoles(req),
  );
  const [memberSkills, { refetch: refetchSkills }] = createResource(
    () => ({ houseId: props.houseId, memberId: props.member.memberId }),
    async (req) => skillClient.listMemberSkills(req),
  );

  const isDeactivated = () => !!props.member.deactivatedAt;

  const deactivateMember = async () => {
    if (!confirm(`Deactivate ${displayName(props.member)}? They'll be denied future login, but their record and content stay and you can reactivate them later.`)) return;
    try {
      await memberClient.deactivateMember(props.member.memberId);
      await props.onChanged();
    } catch (e) { alert(e instanceof Error ? e.message : String(e)); }
  };

  const reactivateMember = async () => {
    try {
      await memberClient.reactivateMember(props.member.memberId);
      await props.onChanged();
    } catch (e) { alert(e instanceof Error ? e.message : String(e)); }
  };

  return (
    <div style="padding:14px 0;border-bottom:1px solid var(--line)">
      <div class={`folk ${status() === "away" ? "away" : ""}`} style="padding:0;border:0">
        <span class={`a lg ${memberSwatch(props.member.memberId)}`}>{initial(props.member)}</span>
        <div style="flex:1;min-width:0">
          <Show
            when={!editing()}
            fallback={
              <DisplayNameEditor
                member={props.member}
                onCancel={() => setEditing(false)}
                onSaved={async () => {
                  setEditing(false);
                  await props.onChanged();
                  if (isSelf()) { try { await loadHouses(); } catch { /* tolerated */ } }
                }}
              />
            }
          >
            <div class="who-name">
              {displayName(props.member)}
              <Show when={isSelf()}>
                <span style="font-size:11px;font-weight:400;color:var(--ink-mute);margin-left:8px">you</span>
              </Show>
              <Show when={isDeactivated()}>
                <span style="font-size:11px;font-weight:400;color:var(--rust);margin-left:8px">deactivated</span>
              </Show>
            </div>
            <div class="doing">
              {props.member.linkkeysUserId}@{props.member.linkkeysDomain}
            </div>
          </Show>
        </div>
        <Show when={!editing() && canEdit()}>
          <button class="btn-quiet" onClick={() => setEditing(true)} style="font-size:12px;padding:4px 10px">
            {isSelf() ? "Edit profile" : "Edit"}
          </button>
        </Show>
        <Show when={!editing() && canAdmin() && !isSelf()}>
          <Show
            when={isDeactivated()}
            fallback={
              <button class="btn-quiet" onClick={deactivateMember} style="font-size:12px;padding:4px 10px;color:var(--rust)">
                Deactivate
              </button>
            }
          >
            <button class="btn-quiet" onClick={reactivateMember} style="font-size:12px;padding:4px 10px;color:var(--grass-4)">
              Reactivate
            </button>
          </Show>
        </Show>
        <Show when={!editing()}>
          <span class="ago">{lastSeenLabel(props.member) ?? ""}</span>
        </Show>
      </div>

      <div style="margin-top:8px;display:grid;grid-template-columns:80px 1fr;gap:6px 12px;align-items:start;font-size:13px">
        <span style="color:var(--ink-mute)">Roles</span>
        <TagChips
          assigned={memberRoles() ?? []}
          all={props.houseRoles}
          getId={(r) => r.roleId}
          getName={(r) => r.name}
          canEdit={canAdmin()}
          color="var(--ocean-2)"
          onAdd={async (roleId) => {
            await roleClient.grantRole({ memberId: props.member.memberId, roleId });
            await refetchRoles();
          }}
          onRemove={async (roleId) => {
            await roleClient.revokeRole({ memberId: props.member.memberId, roleId });
            await refetchRoles();
          }}
        />

        <span style="color:var(--ink-mute)">Skills</span>
        <TagChips
          assigned={memberSkills() ?? []}
          all={props.houseSkills}
          getId={(s) => s.skillId}
          getName={(s) => s.name}
          canEdit={canEdit()}
          color="var(--grass-4)"
          onAdd={async (skillId) => {
            await skillClient.addMemberSkill({ memberId: props.member.memberId, skillId });
            await refetchSkills();
          }}
          onRemove={async (skillId) => {
            await skillClient.removeMemberSkill({ memberId: props.member.memberId, skillId });
            await refetchSkills();
          }}
        />
      </div>
    </div>
  );
};

// Edits the profile fields a member owns: display name and avatar URL. Email is
// shown read-only — it's seeded from a verified linkkeys claim and isn't
// user-settable here (verification territory). The avatar URL is user-
// overridable; the api fetches + caches the image it points at.
const DisplayNameEditor = (props: {
  member: Member;
  onCancel: () => void;
  onSaved: () => Promise<void> | void;
}) => {
  const [name, setName] = createSignal(props.member.displayName ?? "");
  const [avatar, setAvatar] = createSignal(props.member.avatarUrl ?? "");
  const [busy, setBusy] = createSignal(false);
  const [err, setErr] = createSignal<string | null>(null);
  const submit = async (e: SubmitEvent) => {
    e.preventDefault();
    if (busy()) return;
    setBusy(true);
    setErr(null);
    try {
      await memberClient.updateMember({
        ...props.member,
        displayName: name().trim() || undefined,
        avatarUrl: avatar().trim() || undefined,
      } as any);
      await props.onSaved();
    } catch (e2) { setErr(e2 instanceof Error ? e2.message : String(e2)); }
    finally { setBusy(false); }
  };
  const field = "padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px";
  return (
    <form onSubmit={submit} style="display:flex;gap:8px;align-items:center;flex-wrap:wrap">
      <input
        type="text" value={name()} onInput={(e) => setName(e.currentTarget.value)}
        placeholder="display name" autofocus
        style={`flex:1 1 200px;${field}`}
      />
      <input
        type="url" value={avatar()} onInput={(e) => setAvatar(e.currentTarget.value)}
        placeholder="avatar image URL"
        style={`flex:1 1 200px;${field}`}
      />
      <button class="btn btn-primary" disabled={busy()} type="submit" style="padding:6px 14px">
        {busy() ? "…" : "Save"}
      </button>
      <button type="button" class="btn btn-ghost" onClick={props.onCancel} disabled={busy()} style="padding:6px 12px">
        Cancel
      </button>
      <Show when={props.member.email}>
        {(em) => <span style="color:var(--ink-mute);font-size:12px;flex:1 1 100%">{em()} · from linkkeys</span>}
      </Show>
      <Show when={err()}>{(m) => <span style="color:var(--rust);font-size:12px;flex:1 1 100%">{m()}</span>}</Show>
    </form>
  );
};

// ─── Generic chip-set editor ──────────────────────────────────────────

function TagChips<T>(props: {
  assigned: T[];
  all: T[];
  getId: (x: T) => string;
  getName: (x: T) => string;
  canEdit: boolean;
  color: string;
  onAdd: (id: string) => Promise<unknown>;
  onRemove: (id: string) => Promise<unknown>;
}) {
  const [selected, setSelected] = createSignal("");
  const [busy, setBusy] = createSignal(false);

  const available = createMemo(() => {
    const taken = new Set(props.assigned.map(props.getId));
    return props.all.filter((x) => !taken.has(props.getId(x)));
  });

  const add = async () => {
    const id = selected();
    if (!id || busy()) return;
    setBusy(true);
    try {
      await props.onAdd(id);
      setSelected("");
    } catch (e) { alert(e instanceof Error ? e.message : String(e)); }
    finally { setBusy(false); }
  };

  const remove = async (id: string) => {
    if (busy()) return;
    setBusy(true);
    try { await props.onRemove(id); }
    catch (e) { alert(e instanceof Error ? e.message : String(e)); }
    finally { setBusy(false); }
  };

  return (
    <div style="display:flex;flex-wrap:wrap;gap:6px;align-items:center">
      <Show
        when={props.assigned.length > 0}
        fallback={<span style="color:var(--ink-mute);font-size:12px;font-style:italic">none yet</span>}
      >
        <For each={props.assigned}>
          {(item) => (
            <span style={`display:inline-flex;align-items:center;gap:4px;padding:2px 8px;font-size:12px;border-radius:9999px;border:1px solid ${props.color};background:color-mix(in oklab, var(--paper) 88%, ${props.color} 12%);color:var(--ink)`}>
              {props.getName(item)}
              <Show when={props.canEdit}>
                <button
                  type="button"
                  onClick={() => remove(props.getId(item))}
                  style="background:transparent;border:0;color:var(--ink-mute);cursor:pointer;font-size:13px;line-height:1;padding:0"
                  title="Remove"
                  disabled={busy()}
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
          <option value="">+ add</option>
          <For each={available()}>{(x) => <option value={props.getId(x)}>{props.getName(x)}</option>}</For>
        </select>
        <Show when={selected()}>
          <button type="button" class="btn-quiet" onClick={add} disabled={busy()} style="font-size:11px;padding:2px 8px">
            add
          </button>
        </Show>
      </Show>
    </div>
  );
}
