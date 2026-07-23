import type { GapPositionsResponse } from "@/types/domain";

import { MOCK_GRAPH } from "./graph";

export function getGapFixture(): GapPositionsResponse {
  return {
    gapNodeIds: MOCK_GRAPH.nodes.filter((n) => n.data.gap).map((n) => n.id),
  };
}
