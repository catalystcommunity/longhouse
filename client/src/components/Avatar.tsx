import type { Component } from "solid-js";
import type { Member } from "~/data/types";

type Size = "sm" | "md" | "lg";

interface Props {
  member: Pick<Member, "initials" | "swatch">;
  size?: Size;
  class?: string;
}

export const Avatar: Component<Props> = (props) => (
  <span class={`a ${props.member.swatch} ${props.size ?? "md"} ${props.class ?? ""}`}>
    {props.member.initials}
  </span>
);

interface StackProps {
  members: Pick<Member, "initials" | "swatch">[];
  size?: Size;
  max?: number;
}

export const AvatarStack: Component<StackProps> = (props) => {
  const max = () => props.max ?? props.members.length;
  return (
    <div class="who-mini">
      {props.members.slice(0, max()).map((m) => (
        <Avatar member={m} size={props.size ?? "sm"} />
      ))}
    </div>
  );
};
