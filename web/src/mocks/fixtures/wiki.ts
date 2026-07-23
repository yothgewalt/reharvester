import type { GraphNode } from "@/types/domain";



export const RICH_DOCS: Record<string, string> = {
  "attention-is-all-you-need": `# Attention Is All You Need

**Context:** This seminal work replaces recurrence entirely with attention, defining the
transformer architecture that every model in this corpus builds on.

Core Nodes: [[Self-Attention]] -> [[Multi-Head Attention]] -> [[Positional Encoding]]

## Why it matters here

The harvester keeps rediscovering this paper as the citation root of the
<mark>transformer-architectures</mark> cluster — nearly every 2023+ node in the canvas
reaches it within two hops.

## Key mechanisms

| Mechanism | Replaces | Cost |
| --- | --- | --- |
| [[Self-Attention]] | Recurrence | O(n²) in sequence length |
| [[Multi-Head Attention]] | Single similarity space | 8× parallel heads |
| [[Positional Encoding]] | Order from recurrence | Free at inference |

## Open threads

- [[Sparse Attention]] attacks the quadratic term
- [[KV Caching]] turns autoregression practical
- Scaling behavior is formalized in [[Scaling Laws for Neural Language Models]]
`,
  "lora-low-rank-adaptation-of-large-language-models": `# LoRA: Low-Rank Adaptation of Large Language Models

**Context:** Freezes pretrained weights and injects trainable rank-decomposition matrices,
cutting trainable parameters by ~10,000× for downstream tasks.

Core Nodes: [[Rank Decomposition]] -> [[Parameter-Efficient Fine-Tuning]]

## Mechanics

The update is constrained to \`W + BA\` where \`B ∈ R^{d×r}\`, \`A ∈ R^{r×k}\`, \`r ≪ min(d,k)\`.
No inference latency: \`BA\` merges into \`W\` after training.

## Relations

- Quantized follow-up: [[QLoRA: Efficient Finetuning of Quantized LLMs]]
- Alternatives in this cluster: [[Adapter Layers]], [[Prefix Tuning]], [[Prompt Tuning]]
- Failure mode under continual updates: [[Catastrophic Forgetting]]
`,
  "generative-agents-interactive-simulacra-of-human-behavior": `# Generative Agents in Sandbox

**Context:** Populates a small town with LLM-driven agents that remember, reflect, and plan —
the reference architecture for the agentic cluster.

Core Nodes: [[Memory Architectures]] -> [[Self-Reflection]] -> [[Planning Loops]]

## Architecture

1. **Observe** — events stream into a memory log
2. **Reflect** — periodic synthesis into higher-level [[Self-Reflection]] records
3. **Plan** — day-level plans decomposed via [[Task Decomposition]]

## Connected work

Grounding actions with [[Tool Use]] follows [[ReAct: Synergizing Reasoning and Acting]];
evaluation happens on [[Agent Benchmarks]].
`,
  "toy-models-of-superposition": `# Toy Models of Superposition

**Context:** Shows that networks store more features than dimensions by packing them into
non-orthogonal directions — the phenomenon driving the interpretability agenda.

Core Nodes: [[Superposition]] -> [[Sparse Autoencoders]] -> [[Dictionary Learning]]

## The claim

When features are sparse, models exploit interference-tolerant packing. Decoding these
packed features motivates [[Sparse Autoencoders]] and, downstream,
[[Towards Monosemanticity]].

## Toolchain in this cluster

- [[Activation Patching]] for causal claims
- [[Circuit Analysis]] for mechanism-level maps
- [[Probing Classifiers]] as the baseline lens
`,
  "retrieval-augmented-generation-for-knowledge-intensive-nlp": `# Retrieval-Augmented Generation for Knowledge-Intensive NLP

**Context:** Couples a dense retriever with a generator so knowledge can be swapped by
re-indexing instead of retraining.

Core Nodes: [[Dense Passage Retrieval]] -> [[Vector Databases]] -> [[Reranking]]

## Pipeline

Query → [[Query Expansion]] → retrieve ([[Hybrid Search]]) → [[Reranking]] →
ground the generator with [[Citation Grounding]] to fight [[Hallucination Mitigation]] targets.
`,
};


export function generateWikiMarkdown(
  node: GraphNode,
  neighbors: Array<{ id: string; label: string }>,
): string {
  const links = neighbors
    .slice(0, 6)
    .map((n) => `[[${n.id}|${n.label}]]`)
    .join(" · ");
  const kindLine =
    node.data.kind === "paper"
      ? `Registered paper (${node.data.year}) in the **${node.data.cluster}** cluster.`
      : `Concept node in the **${node.data.cluster}** cluster.`;
  return `# ${node.label}

**Context:** ${kindLine}

PageRank ${node.data.pageRank.toFixed(4)} · ${node.data.openAccess ? "Open access" : "Restricted access"}

## Connected nodes

${links || "_No recorded connections yet._"}

## Notes

_The local harvester has not synthesized notes for this entry yet. Trigger a crawl to
enrich it._
`;
}

export function stubWikiMarkdown(docId: string): string {
  return `# ${docId}

_No wiki entry for this concept yet._
`;
}
