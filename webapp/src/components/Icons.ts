import { type SvgAttributes, el } from "~/ui/dom";
/** Inline SVG icons. All use currentColor so they inherit from CSS. */
type IconProps = SvgAttributes;
const base = {
    viewBox: "0 0 24 24",
    fill: "none",
    stroke: "currentColor",
    "stroke-width": 1.7,
    "stroke-linecap": "round",
    "stroke-linejoin": "round",
} as const;
export const LonghouseMark = (p: IconProps = {}) => (el("svg", {
    ...base,
    ...p,
    get children() {
        return [
            el("path", {
                "d": "M3 11 L12 4 L21 11"
            }),
            el("path", {
                "d": "M5 11 V20 H19 V11"
            }),
            el("path", {
                "d": "M10 20 V14 H14 V20"
            }),
            el("path", {
                "d": "M16 4 Q17 6 16 7 Q15 8 16 9"
            })
        ];
    }
}));
export const Search = (p: IconProps = {}) => (el("svg", {
    ...base,
    ...p,
    get children() {
        return [
            el("circle", {
                "cx": "11",
                "cy": "11",
                "r": "6.5"
            }),
            el("path", {
                "d": "m20 20-3.5-3.5"
            })
        ];
    }
}));
export const Bell = (p: IconProps = {}) => (el("svg", {
    ...base,
    ...p,
    get children() {
        return [
            el("path", {
                "d": "M6 9a6 6 0 1 1 12 0c0 5 2 6 2 6H4s2-1 2-6Z"
            }),
            el("path", {
                "d": "M10 19a2 2 0 0 0 4 0"
            })
        ];
    }
}));
export const Sun = (p: IconProps = {}) => (el("svg", {
    ...base,
    ...p,
    get children() {
        return [
            el("circle", {
                "cx": "12",
                "cy": "12",
                "r": "4.2"
            }),
            el("path", {
                "d": "M12 3v2M12 19v2M3 12h2M19 12h2M5.6 5.6l1.4 1.4M17 17l1.4 1.4M5.6 18.4 7 17M17 7l1.4-1.4"
            })
        ];
    }
}));
export const Moon = (p: IconProps = {}) => (el("svg", {
    ...base,
    ...p,
    get children() {
        return el("path", {
            "d": "M20 14.5A8 8 0 1 1 9.5 4a6.5 6.5 0 0 0 10.5 10.5Z"
        });
    }
}));
export const Landscape = (p: IconProps = {}) => (el("svg", {
    ...base,
    ...p,
    get children() {
        return [
            el("rect", {
                "x": "3",
                "y": "5",
                "width": "18",
                "height": "14",
                "rx": "2"
            }),
            el("circle", {
                "cx": "8",
                "cy": "10",
                "r": "1.6"
            }),
            el("path", {
                "d": "m3 17 5-5 4 3 3-3 6 6"
            })
        ];
    }
}));
export const Pin = (p: IconProps = {}) => (el("svg", {
    ...base,
    ...p,
    get children() {
        return [
            el("path", {
                "d": "M12 22s-7-7.5-7-13a7 7 0 1 1 14 0c0 5.5-7 13-7 13Z"
            }),
            el("circle", {
                "cx": "12",
                "cy": "9",
                "r": "2.5"
            })
        ];
    }
}));
export const ChevronLeft = (p: IconProps = {}) => (el("svg", {
    ...base,
    ...p,
    get children() {
        return el("path", {
            "d": "m14 6-6 6 6 6"
        });
    }
}));
export const ChevronRight = (p: IconProps = {}) => (el("svg", {
    ...base,
    ...p,
    get children() {
        return el("path", {
            "d": "m10 6 6 6-6 6"
        });
    }
}));
export const Plus = (p: IconProps = {}) => (el("svg", {
    ...base,
    "viewBox": "0 0 16 16",
    "stroke-width": 2,
    ...p,
    get children() {
        return el("path", {
            "d": "M8 3v10M3 8h10"
        });
    }
}));
export const Check = (p: IconProps = {}) => (el("svg", {
    "viewBox": "0 0 16 16",
    "fill": "none",
    "stroke": "currentColor",
    "stroke-width": "2.4",
    "stroke-linecap": "round",
    "stroke-linejoin": "round",
    ...p,
    get children() {
        return el("path", {
            "d": "m3 8 3.5 3.5L13 5"
        });
    }
}));
export const Home = (p: IconProps = {}) => (el("svg", {
    ...base,
    ...p,
    get children() {
        return [
            el("path", {
                "d": "M3 11 12 4l9 7"
            }),
            el("path", {
                "d": "M5 11v9h14v-9"
            })
        ];
    }
}));
export const Calendar = (p: IconProps = {}) => (el("svg", {
    ...base,
    ...p,
    get children() {
        return [
            el("rect", {
                "x": "4",
                "y": "5",
                "width": "16",
                "height": "15",
                "rx": "2"
            }),
            el("path", {
                "d": "M8 3v4M16 3v4M4 10h16"
            })
        ];
    }
}));
export const Clock = (p: IconProps = {}) => (el("svg", {
    ...base,
    ...p,
    get children() {
        return [
            el("circle", {
                "cx": "12",
                "cy": "12",
                "r": "9"
            }),
            el("path", {
                "d": "M12 7v5l3 2"
            })
        ];
    }
}));
export const People = (p: IconProps = {}) => (el("svg", {
    ...base,
    ...p,
    get children() {
        return [
            el("circle", {
                "cx": "9",
                "cy": "9",
                "r": "3.5"
            }),
            el("circle", {
                "cx": "17",
                "cy": "11",
                "r": "2.5"
            }),
            el("path", {
                "d": "M3 20c0-3 3-5 6-5s6 2 6 5"
            }),
            el("path", {
                "d": "M14 19c0-2 1.5-3.5 3.5-3.5S21 17 21 19"
            })
        ];
    }
}));
export const Menu = (p: IconProps = {}) => (el("svg", {
    ...base,
    ...p,
    get children() {
        return el("path", {
            "d": "M4 7h16M4 12h16M4 17h10"
        });
    }
}));
export const Cloud = (p: IconProps = {}) => (el("svg", {
    ...base,
    ...p,
    get children() {
        return [
            el("path", {
                "d": "M7 17h10a3.5 3.5 0 0 0 0-7 5 5 0 0 0-9.6-1.3A3.5 3.5 0 0 0 7 17Z"
            }),
            el("circle", {
                "cx": "17",
                "cy": "6",
                "r": "2.4",
                "fill": "currentColor",
                "opacity": ".35",
                "stroke": "none"
            })
        ];
    }
}));
export const Bug = (p: IconProps = {}) => (el("svg", {
    ...base,
    ...p,
    get children() {
        return [
            el("ellipse", {
                "cx": "12",
                "cy": "13",
                "rx": "5",
                "ry": "6"
            }),
            el("path", {
                "d": "M12 7V5a3 3 0 0 0-3-3M12 7V5a3 3 0 0 1 3-3"
            }),
            el("path", {
                "d": "M7 11H4M20 11h-3M7 16H3M21 16h-4M9 19l-2 3M15 19l2 3"
            })
        ];
    }
}));
