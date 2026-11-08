import { observe, query, cleanup } from "~/ui/state";
import { type View, When, el, live } from "~/ui/dom";
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
    if (!tok)
        return null;
    try {
        const res = await fetch(`/api/v1/avatars/${memberId}`, {
            headers: { Authorization: `Bearer ${tok}` },
        });
        if (!res.ok)
            return null;
        return URL.createObjectURL(await res.blob());
    }
    catch {
        return null;
    }
}
export const Avatar: View<Props> = (props) => {
    // Only fetches when avatarMemberId is set (i.e. the member has an avatar_url).
    const [url] = query(() => props.bits.avatarMemberId, loadAvatar);
    // Revoke each object URL when it's replaced or the component unmounts.
    observe(() => {
        const u = url();
        if (u)
            cleanup(() => URL.revokeObjectURL(u));
    });
    return (el("span", {
        get "class"() {
            return `a ${props.bits.swatch} ${props.size ?? "md"} ${props.class ?? ""}`;
        },
        get "title"() {
            return props.bits.name;
        },
        get children() {
            return When({
                get "when"() {
                    return url();
                },
                get "fallback"() {
                    return props.bits.initials;
                },
                children: (u) => el("img", {
                    "class": "a-img",
                    get "src"() {
                        return u();
                    },
                    get "alt"() {
                        return props.bits.name ?? "";
                    }
                })
            });
        }
    }));
};
interface StackProps {
    bits: AvatarBits[];
    size?: Size;
    max?: number;
}
export const AvatarStack: View<StackProps> = (props) => {
    const max = () => props.max ?? props.bits.length;
    return (el("div", {
        "class": "who-mini",
        get children() {
            return live(() => props.bits.slice(0, max()).map((b) => (Avatar({
                get "bits"() {
                    return b;
                },
                get "size"() {
                    return props.size ?? "sm";
                }
            }))));
        }
    }));
};
