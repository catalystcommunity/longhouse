import type { Component } from "solid-js";
import type { AvatarBits } from "~/lib/derive";

type Size = "sm" | "md" | "lg";

interface Props {
  bits: AvatarBits;
  size?: Size;
  class?: string;
}

export const Avatar: Component<Props> = (props) => (
  <span class={`a ${props.bits.swatch} ${props.size ?? "md"} ${props.class ?? ""}`}>
    {props.bits.initials}
  </span>
);

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
