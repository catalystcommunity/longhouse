import { type Component, createEffect, createResource, onCleanup, Show } from "solid-js";
import type { AvatarBits } from "~/lib/derive";
import { currentToken } from "~/stores/auth";

type Size = "sm" | "md" | "lg";

interface Props {
  bits: AvatarBits;
  size?: Size;
  class?: string;
}

// loadAvatar fetches the cached avatar bytes with the bearer (an <img src> can't
// carry the Authorization header) and hands back an object URL. Any failure —
// no token, 404 (no cached image / outside the house), network — resolves to
// null so the component falls back to initials.
async function loadAvatar(memberId: string): Promise<string | null> {
  const tok = currentToken();
  if (!tok) return null;
  try {
    const res = await fetch(`/api/v1/avatars/${memberId}`, {
      headers: { Authorization: `Bearer ${tok}` },
    });
    if (!res.ok) return null;
    return URL.createObjectURL(await res.blob());
  } catch {
    return null;
  }
}

export const Avatar: Component<Props> = (props) => {
  // Only fetches when avatarMemberId is set (i.e. the member has an avatar_url).
  const [url] = createResource(() => props.bits.avatarMemberId, loadAvatar);
  // Revoke each object URL when it's replaced or the component unmounts.
  createEffect(() => {
    const u = url();
    if (u) onCleanup(() => URL.revokeObjectURL(u));
  });
  return (
    <span
      class={`a ${props.bits.swatch} ${props.size ?? "md"} ${props.class ?? ""}`}
      title={props.bits.name}
    >
      <Show when={url()} fallback={props.bits.initials}>
        {(u) => <img class="a-img" src={u()} alt={props.bits.name ?? ""} />}
      </Show>
    </span>
  );
};

interface StackProps {
  bits: AvatarBits[];
  size?: Size;
  max?: number;
}

export const AvatarStack: Component<StackProps> = (props) => {
  const max = () => props.max ?? props.bits.length;
  return (
    <div class="who-mini">
      {props.bits.slice(0, max()).map((b) => (
        <Avatar bits={b} size={props.size ?? "sm"} />
      ))}
    </div>
  );
};
