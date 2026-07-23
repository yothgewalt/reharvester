

import { darkTheme } from "reagraph";
import type { Theme } from "reagraph";


export const graphTheme: Theme = {
  ...darkTheme,
  canvas: { ...darkTheme.canvas, background: "#0F0F10" },
  node: {
    ...darkTheme.node,
    fill: "#66707A",
    activeFill: "#E4E6E7",
    inactiveOpacity: 0.15,
    label: {
      ...darkTheme.node.label,
      color: "#959A9D",
      activeColor: "#E4E6E7",
      stroke: "#0F0F10",
    },
  },
  ring: { fill: "#4E5355", activeFill: "#E4E6E7" },
  edge: {
    ...darkTheme.edge,
    fill: "#26282A",
    activeFill: "#959A9D",
    inactiveOpacity: 0.08,
    label: {
      ...darkTheme.edge.label,
      color: "#959A9D",
      activeColor: "#E4E6E7",
      stroke: "#0F0F10",
      fontSize: 6,
    },
  },
  arrow: { fill: "#26282A", activeFill: "#959A9D" },
};
