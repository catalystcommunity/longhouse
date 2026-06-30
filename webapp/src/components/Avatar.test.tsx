import { describe, expect, it } from "vitest";
import { render } from "@solidjs/testing-library";
import { Avatar, AvatarStack } from "./Avatar";

describe("Avatar", () => {
  it("renders initials and the swatch + size classes", () => {
    const { container } = render(() => (
      <Avatar bits={{ initials: "T", swatch: "a1" }} size="lg" />
    ));
    const el = container.querySelector(".a")!;
    expect(el.textContent).toBe("T");
    expect(el.classList.contains("a1")).toBe(true);
    expect(el.classList.contains("lg")).toBe(true);
  });

  it("defaults to medium size", () => {
    const { container } = render(() => (
      <Avatar bits={{ initials: "L", swatch: "a2" }} />
    ));
    expect(container.querySelector(".a")!.classList.contains("md")).toBe(true);
  });
});

describe("AvatarStack", () => {
  const bits = [
    { initials: "T", swatch: "a1" as const },
    { initials: "L", swatch: "a2" as const },
    { initials: "S", swatch: "a3" as const },
  ];

  it("renders one avatar per member", () => {
    const { container } = render(() => <AvatarStack bits={bits} />);
    expect(container.querySelectorAll(".who-mini .a")).toHaveLength(3);
  });

  it("caps at max", () => {
    const { container } = render(() => <AvatarStack bits={bits} max={2} />);
    expect(container.querySelectorAll(".who-mini .a")).toHaveLength(2);
  });
});
