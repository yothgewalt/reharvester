"use client";

const PROMPTS = [
  "What are the main research areas here?",
  "Which research gaps look most promising?",
  "What topics are emerging recently?",
  "Which papers are most influential?",
];

export function StarterPrompts({ disabled, onPick }: { disabled: boolean; onPick(prompt: string): void }) {
  return (
    <div className="flex h-full flex-col items-start justify-center gap-3">
      <p className="m-0 text-[13px] text-ink-3">Try asking:</p>
      <div className="flex flex-wrap gap-2">
        {PROMPTS.map((p) => (
          <button
            key={p}
            type="button"
            disabled={disabled}
            onClick={() => onPick(p)}
            className="rounded-md border border-line px-3 py-2 text-left text-[13px] text-ink-2 transition-colors duration-150 hover:bg-bg-subtle disabled:cursor-not-allowed disabled:opacity-50"
          >
            {p}
          </button>
        ))}
      </div>
    </div>
  );
}
