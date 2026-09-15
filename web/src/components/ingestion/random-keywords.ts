import { transport } from "@/lib/api/transport";

interface Topic {
  field: string;
  keywords: readonly string[];
}

/** Each entry is one coherent topic that passes keyword-mode validation on its own. */
export const TOPICS: readonly Topic[] = [
  { field: "astrophysics", keywords: ["dark matter", "wimp", "axion", "direct detection", "galactic halo"] },
  { field: "cosmology", keywords: ["cosmic microwave background", "inflation", "hubble tension", "baryon acoustic oscillations", "dark energy"] },
  { field: "gravitational physics", keywords: ["gravitational waves", "binary black holes", "ligo", "waveform modeling", "neutron star mergers"] },
  { field: "quantum computing", keywords: ["quantum error correction", "surface code", "logical qubits", "fault tolerance", "decoding algorithms"] },
  { field: "condensed matter physics", keywords: ["topological insulators", "majorana fermions", "quantum anomalous hall", "berry phase", "edge states"] },
  { field: "machine learning", keywords: ["diffusion models", "score matching", "denoising", "image generation", "sampling efficiency"] },
  { field: "natural language processing", keywords: ["retrieval augmented generation", "dense retrieval", "large language models", "hallucination", "question answering"] },
  { field: "computer vision", keywords: ["neural radiance fields", "novel view synthesis", "3d reconstruction", "gaussian splatting", "volume rendering"] },
  { field: "reinforcement learning", keywords: ["offline reinforcement learning", "policy optimization", "reward modeling", "distribution shift", "model-based planning"] },
  { field: "robotics", keywords: ["legged locomotion", "sim-to-real transfer", "motion planning", "whole-body control", "terrain adaptation"] },
  { field: "computational biology", keywords: ["protein structure prediction", "alphafold", "protein language models", "molecular dynamics", "drug discovery"] },
  { field: "cryptography", keywords: ["post-quantum cryptography", "lattice-based schemes", "learning with errors", "key encapsulation", "side-channel attacks"] },
  { field: "distributed systems", keywords: ["consensus protocols", "byzantine fault tolerance", "state machine replication", "raft", "blockchain scalability"] },
  { field: "quantitative finance", keywords: ["portfolio optimization", "volatility forecasting", "market microstructure", "risk parity", "high-frequency trading"] },
  { field: "climate science", keywords: ["climate modeling", "extreme precipitation", "downscaling", "sea level rise", "earth system models"] },
];

/**
 * Keywords for a random topic. Uses the local model when `useModel` is true and
 * it answers with a full set; otherwise the topic's built-in keywords. Never rejects.
 */
export async function randomKeywords(useModel: boolean, random: () => number = Math.random): Promise<string[]> {
  const topic = TOPICS[Math.floor(random() * TOPICS.length)];
  if (!useModel) return [...topic.keywords];
  try {
    const generated = await transport.randomKeywords(topic.field);
    return generated.length >= topic.keywords.length ? generated : [...topic.keywords];
  } catch {
    return [...topic.keywords];
  }
}
