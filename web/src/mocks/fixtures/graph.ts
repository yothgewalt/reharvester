import type { GraphEdge, GraphNode, GraphSnapshot } from "@/types/domain";


function mulberry32(seed: number) {
  return function () {
    let t = (seed += 0x6d2b79f5);
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

export const slug = (s: string) =>
  s
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");

interface ClusterSpec {
  id: string;
  hubs: string[];
  terms: string[];
}

export const CLUSTER_SPECS: ClusterSpec[] = [
  {
    id: "transformer-architectures",
    hubs: ["Attention Is All You Need", "Scaling Laws for Neural Language Models"],
    terms: [
      "Self-Attention",
      "Multi-Head Attention",
      "Positional Encoding",
      "Sparse Attention",
      "KV Caching",
      "Rotary Embeddings",
      "Context Window Scaling",
      "Mixture of Experts",
      "Flash Attention",
      "Layer Normalization",
    ],
  },
  {
    id: "graph-neural-networks",
    hubs: ["Semi-Supervised Classification with Graph Convolutional Networks", "Graph Attention Networks"],
    terms: [
      "Message Passing",
      "Node Embeddings",
      "Graph Pooling",
      "Spectral Convolution",
      "Oversmoothing",
      "Heterogeneous Graphs",
      "Temporal Graphs",
      "Graph Transformers",
      "Link Prediction",
      "Knowledge Graph Completion",
    ],
  },
  {
    id: "low-rank-adaptation",
    hubs: ["LoRA: Low-Rank Adaptation of Large Language Models", "QLoRA: Efficient Finetuning of Quantized LLMs"],
    terms: [
      "Parameter-Efficient Fine-Tuning",
      "Adapter Layers",
      "Rank Decomposition",
      "Quantization-Aware Training",
      "Prefix Tuning",
      "Prompt Tuning",
      "Delta Weights",
      "Catastrophic Forgetting",
      "Model Merging",
      "Weight Tying",
    ],
  },
  {
    id: "retrieval-augmented-generation",
    hubs: ["Retrieval-Augmented Generation for Knowledge-Intensive NLP", "Dense Passage Retrieval"],
    terms: [
      "Vector Databases",
      "Chunking Strategies",
      "Hybrid Search",
      "Reranking",
      "Embedding Models",
      "Context Compression",
      "Hallucination Mitigation",
      "Citation Grounding",
      "Query Expansion",
      "Late Interaction",
    ],
  },
  {
    id: "agentic-systems",
    hubs: ["Generative Agents: Interactive Simulacra of Human Behavior", "ReAct: Synergizing Reasoning and Acting"],
    terms: [
      "Tool Use",
      "Planning Loops",
      "Multi-Agent Coordination",
      "Memory Architectures",
      "Self-Reflection",
      "Task Decomposition",
      "Environment Grounding",
      "Agent Benchmarks",
      "Sandboxed Execution",
      "Human-in-the-Loop",
    ],
  },
  {
    id: "mechanistic-interpretability",
    hubs: ["Toy Models of Superposition", "Towards Monosemanticity"],
    terms: [
      "Sparse Autoencoders",
      "Circuit Analysis",
      "Feature Visualization",
      "Activation Patching",
      "Induction Heads",
      "Probing Classifiers",
      "Superposition",
      "Dictionary Learning",
      "Causal Tracing",
      "Logit Lens",
    ],
  },
  {
    id: "federated-learning",
    hubs: ["Communication-Efficient Learning of Deep Networks from Decentralized Data", "Advances and Open Problems in Federated Learning"],
    terms: [
      "FedAvg",
      "Differential Privacy",
      "Secure Aggregation",
      "Client Drift",
      "Personalization",
      "Gradient Compression",
      "Cross-Silo Training",
      "Byzantine Robustness",
      "Split Learning",
      "On-Device Training",
    ],
  },
  {
    id: "diffusion-models",
    hubs: ["Denoising Diffusion Probabilistic Models", "High-Resolution Image Synthesis with Latent Diffusion"],
    terms: [
      "Score Matching",
      "Noise Scheduling",
      "Classifier-Free Guidance",
      "Latent Spaces",
      "Consistency Models",
      "Flow Matching",
      "Inpainting",
      "ControlNets",
      "Video Diffusion",
      "Distillation Sampling",
    ],
  },
  {
    id: "neuro-symbolic-reasoning",
    hubs: ["Neural Theorem Provers", "Chain-of-Thought Prompting Elicits Reasoning"],
    terms: [
      "Program Synthesis",
      "Symbolic Execution",
      "Logic Constraints",
      "Formal Verification",
      "Proof Search",
      "Semantic Parsing",
      "Abstract Reasoning",
      "Rule Induction",
      "Constraint Satisfaction",
      "Process Supervision",
    ],
  },
];

const PAPER_PREFIXES = [
  "Benchmarking",
  "Rethinking",
  "A Survey of",
  "On the Limits of",
  "Scaling",
  "Understanding",
  "Efficient",
  "Robust",
  "Emergent Properties of",
  "Evaluating",
];

export interface GeneratedGraph {
  nodes: GraphNode[];
  edges: GraphEdge[];
}


export function generateGraph(targetNodes = 450, seed = 1337): GeneratedGraph {
  const rng = mulberry32(seed);
  const nodes: GraphNode[] = [];
  const edges: GraphEdge[] = [];
  const usedIds = new Set<string>();
  const edgeKeys = new Set<string>();

  const uniqueId = (label: string): string => {
    let id = slug(label);
    let n = 2;
    while (usedIds.has(id)) id = `${slug(label)}-${n++}`;
    usedIds.add(id);
    return id;
  };

  const addNode = (label: string, kind: "paper" | "concept", cluster: string): GraphNode => {
    const id = uniqueId(label);
    const node: GraphNode = {
      id,
      label,
      data: {
        kind,
        docId: id,
        year: 2015 + Math.floor(rng() * 12),
        pageRank: 0, // filled after edges
        gap: false, // filled after edges
        cluster,
        openAccess: kind === "paper" ? rng() < 0.65 : false,
      },
    };
    nodes.push(node);
    return node;
  };


  const relationFor = (source: GraphNode, target: GraphNode): string => {
    if (source.data.kind === "paper" && target.data.kind === "paper") return "cites";
    if (source.data.kind === "concept" && target.data.kind === "paper") return "introduced-in";
    if (source.data.kind === "paper" && target.data.kind === "concept") return "proposes";
    return "builds-on";
  };

  const addEdge = (source: string, target: string, weight: number, relation: string) => {
    if (source === target) return;
    const key = `${source}->${target}`;
    if (edgeKeys.has(key) || edgeKeys.has(`${target}->${source}`)) return;
    edgeKeys.add(key);
    edges.push({ id: key, source, target, weight: Math.round(weight * 100) / 100, relation });
  };

  const perCluster = Math.max(6, Math.floor(targetNodes / CLUSTER_SPECS.length));
  const clusterHubs: GraphNode[][] = [];
  const clusterMembers: GraphNode[][] = [];

  for (const spec of CLUSTER_SPECS) {
    const hubs = spec.hubs.map((h) => addNode(h, "paper", spec.id));
    clusterHubs.push(hubs);
    addEdge(hubs[0].id, hubs[1].id, 0.9, "extends");

    const members: GraphNode[] = [];
    const memberCount = perCluster - hubs.length;
    for (let i = 0; i < memberCount; i++) {
      let node: GraphNode;
      if (i < spec.terms.length) {
        node = addNode(spec.terms[i], "concept", spec.id);
      } else {
        const prefix = PAPER_PREFIXES[Math.floor(rng() * PAPER_PREFIXES.length)];
        const term = spec.terms[Math.floor(rng() * spec.terms.length)];
        node = addNode(`${prefix} ${term}`, "paper", spec.id);
      }
      members.push(node);


      const hub = hubs[Math.floor(rng() * hubs.length)];
      addEdge(node.id, hub.id, 0.4 + rng() * 0.6, relationFor(node, hub));

      if (members.length > 1 && rng() < 0.4) {
        const other = members[Math.floor(rng() * (members.length - 1))];
        addEdge(node.id, other.id, 0.2 + rng() * 0.5, relationFor(node, other));
      }
    }
    clusterMembers.push(members);
  }



  for (let c = 0; c < CLUSTER_SPECS.length; c++) {
    const next = (c + 1) % CLUSTER_SPECS.length;
    addEdge(clusterHubs[c][0].id, clusterHubs[next][0].id, 0.3, "influences");
  }
  const bridgeCount = Math.max(4, Math.floor(targetNodes / 90));
  for (let i = 0; i < bridgeCount; i++) {
    const a = Math.floor(rng() * CLUSTER_SPECS.length);
    let b = Math.floor(rng() * CLUSTER_SPECS.length);
    if (b === a) b = (b + 1) % CLUSTER_SPECS.length;
    const from = clusterMembers[a][Math.floor(rng() * clusterMembers[a].length)];
    const to = clusterMembers[b][Math.floor(rng() * clusterMembers[b].length)];
    addEdge(from.id, to.id, 0.15 + rng() * 0.2, "influences");
  }


  const inDeg = new Map<string, number>();
  const totalDeg = new Map<string, number>();
  const interCluster = new Set<string>();
  const clusterOf = new Map(nodes.map((n) => [n.id, n.data.cluster]));
  for (const e of edges) {
    inDeg.set(e.target, (inDeg.get(e.target) ?? 0) + 1);
    totalDeg.set(e.source, (totalDeg.get(e.source) ?? 0) + 1);
    totalDeg.set(e.target, (totalDeg.get(e.target) ?? 0) + 1);
    if (clusterOf.get(e.source) !== clusterOf.get(e.target)) {
      interCluster.add(e.source);
      interCluster.add(e.target);
    }
  }
  const maxIn = Math.max(1, ...inDeg.values());
  for (const n of nodes) {
    n.data.pageRank = Math.round((0.002 + 0.078 * ((inDeg.get(n.id) ?? 0) / maxIn)) * 1e4) / 1e4;

    n.data.gap = interCluster.has(n.id) && (totalDeg.get(n.id) ?? 0) <= 3;
  }

  const gapTarget = Math.floor(nodes.length * 0.07);
  let gapCount = nodes.filter((n) => n.data.gap).length;
  for (const n of nodes) {
    if (gapCount >= gapTarget) break;
    if (!n.data.gap && n.data.kind === "concept" && (totalDeg.get(n.id) ?? 0) <= 2) {
      n.data.gap = true;
      gapCount++;
    }
  }

  return { nodes, edges };
}

export const MOCK_GRAPH: GeneratedGraph = generateGraph(450);


export function generateDeltaBatch(seed = 7331): GeneratedGraph {
  const rng = mulberry32(seed);
  const nodes: GraphNode[] = [];
  const edges: GraphEdge[] = [];
  const hubs = MOCK_GRAPH.nodes.filter((n) => n.data.pageRank > 0.04);
  const existing = new Set(MOCK_GRAPH.nodes.map((n) => n.id));
  for (let i = 0; i < 30; i++) {
    const hub = hubs[Math.floor(rng() * hubs.length)];
    const label = `Harvested: ${PAPER_PREFIXES[i % PAPER_PREFIXES.length]} ${hub.label.split(" ").slice(0, 3).join(" ")} ${i + 1}`;
    let id = slug(label);
    if (existing.has(id)) id = `${id}-x`;
    existing.add(id);
    nodes.push({
      id,
      label,
      data: {
        kind: "paper",
        docId: id,
        year: 2026,
        pageRank: 0.004,
        gap: false,
        cluster: hub.data.cluster,
        openAccess: rng() < 0.65,
      },
    });
    edges.push({ id: `${id}->${hub.id}`, source: id, target: hub.id, weight: 0.5, relation: "cites" });
    if (rng() < 0.5) {
      const other = hubs[Math.floor(rng() * hubs.length)];
      if (other.id !== hub.id) {
        edges.push({
          id: `${id}->${other.id}`,
          source: id,
          target: other.id,
          weight: 0.25,
          relation: "cites",
        });
      }
    }
  }
  return { nodes, edges };
}

export const DELTA_BATCH: GeneratedGraph = generateDeltaBatch();


export function generateCooccurrenceGraph(base: GeneratedGraph = MOCK_GRAPH): GeneratedGraph {
  const concepts = base.nodes.filter((n) => n.data.kind === "concept");
  const conceptIds = new Set(concepts.map((n) => n.id));


  const neighbors = new Map<string, Set<string>>();
  const addNeighbor = (a: string, b: string) => {
    let set = neighbors.get(a);
    if (!set) neighbors.set(a, (set = new Set()));
    set.add(b);
  };
  for (const e of base.edges) {
    addNeighbor(e.source, e.target);
    addNeighbor(e.target, e.source);
  }


  const pairWeight = new Map<string, number>();
  const ids = concepts.map((n) => n.id);
  for (let i = 0; i < ids.length; i++) {
    const na = neighbors.get(ids[i]);
    if (!na) continue;
    for (let j = i + 1; j < ids.length; j++) {
      const nb = neighbors.get(ids[j]);
      if (!nb) continue;
      let shared = 0;
      for (const x of na) if (nb.has(x)) shared++;
      if (shared > 0) pairWeight.set(`${ids[i]}|${ids[j]}`, shared);
    }
  }


  const perNode = new Map<string, Array<{ other: string; weight: number; key: string }>>();
  for (const [key, weight] of pairWeight) {
    const [a, b] = key.split("|");
    (perNode.get(a) ?? perNode.set(a, []).get(a)!).push({ other: b, weight, key });
    (perNode.get(b) ?? perNode.set(b, []).get(b)!).push({ other: a, weight, key });
  }
  const keptKeys = new Set<string>();
  for (const list of perNode.values()) {
    list
      .sort((x, y) => y.weight - x.weight)
      .slice(0, 6)
      .forEach((e) => keptKeys.add(e.key));
  }

  const edges: GraphEdge[] = [...keptKeys].map((key) => {
    const [a, b] = key.split("|");
    return {
      id: `${a}<->${b}`,
      source: a,
      target: b,
      weight: pairWeight.get(key) ?? 1,
      relation: "co-occurs-with", // contract completeness — never rendered
    };
  });


  const frequency = new Map<string, number>();
  for (const e of edges) {
    frequency.set(e.source, (frequency.get(e.source) ?? 0) + e.weight);
    frequency.set(e.target, (frequency.get(e.target) ?? 0) + e.weight);
  }
  const nodes: GraphNode[] = concepts.map((n) => ({
    ...n,
    data: { ...n.data, frequency: frequency.get(n.id) ?? 1 },
  }));

  return { nodes: nodes.filter((n) => conceptIds.has(n.id)), edges };
}

export const MOCK_COOCCURRENCE: GeneratedGraph = generateCooccurrenceGraph();

export function getMockSnapshot(kind: "knowledge" | "cooccurrence" = "knowledge"): GraphSnapshot {
  const graph = kind === "cooccurrence" ? MOCK_COOCCURRENCE : MOCK_GRAPH;
  return {
    nodes: graph.nodes,
    edges: graph.edges,
    generatedAt: "2026-07-22T00:00:00.000Z",
  };
}
