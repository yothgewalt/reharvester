"use client";

import Button from "@mui/material/Button";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";
import { useEffect } from "react";

import { SectionShell } from "@/components/layout/SectionShell";
import { useAppStore } from "@/store";

import { AutosaveField } from "./AutosaveField";
import { SecretField } from "./SecretField";

const OPENALEX_ENV = "OPENALEX_API_KEY";
const SEMANTIC_SCHOLAR_ENV = "S2_API_KEY";
const OLLAMA_ENV = "OLLAMA_API_KEY";

export function SettingsSection() {
  const settings = useAppStore((s) => s.settings);
  const settingsLoadError = useAppStore((s) => s.settingsLoadError);
  const saveStatus = useAppStore((s) => s.saveStatus);
  const loadSettings = useAppStore((s) => s.loadSettings);
  const saveSetting = useAppStore((s) => s.saveSetting);

  useEffect(() => {
    void loadSettings();
  }, [loadSettings]);

  if (!settings) {
    return (
      <SectionShell
        id="settings"
        eyebrow="Local server"
        title="Settings"
        description="Live settings for the running server."
      >
        {settingsLoadError ? (
          <div className="flex flex-col items-start gap-3">
            <Typography role="alert" variant="body2" className="text-error">
              Couldn&apos;t load settings: {settingsLoadError}
            </Typography>
            <Button variant="outlined" onClick={() => void loadSettings()}>
              Retry
            </Button>
          </div>
        ) : (
          <Typography variant="body2" className="text-ink-2">
            Loading settings…
          </Typography>
        )}
      </SectionShell>
    );
  }

  const statusText =
    saveStatus.state === "saving"
      ? "Saving…"
      : saveStatus.state === "saved"
        ? saveStatus.message
          ? `Saved — ${saveStatus.message}`
          : "Saved"
        : saveStatus.state === "error"
          ? (saveStatus.message ?? "Couldn't save.")
          : "";

  return (
    <SectionShell
      id="settings"
      eyebrow="Local server"
      title="Settings"
      description="Every change here applies to the running server immediately."
    >
      <div className="flex flex-col gap-6">
        <p role="status" aria-live="polite" className="min-h-5 font-mono text-[13px] text-ink-2">
          {statusText}
        </p>
        {!settings.persisted ? (
          <p className="text-[13px] text-warn">Changes apply now but last only until this server stops.</p>
        ) : null}

        <fieldset className="flex flex-col gap-4 rounded-md border border-line p-4">
          <legend className="px-1 font-mono text-[13px] uppercase tracking-wide text-ink-2">
            Harvest defaults
          </legend>
          <div className="grid grid-cols-2 gap-4">
            <AutosaveField
              id="settings-source"
              type="select"
              label="Source"
              fieldKey="source"
              value={settings.settings.source}
              options={settings.sources.map((src) => ({ value: src, label: src }))}
              onSave={(v) => saveSetting({ source: v })}
            />
            <AutosaveField
              id="settings-max"
              type="number"
              label="Max records"
              fieldKey="max"
              value={String(settings.settings.max)}
              onSave={(v) => saveSetting({ max: Number(v) })}
            />
            <AutosaveField
              id="settings-delay"
              type="text"
              label="Delay between requests"
              fieldKey="delay"
              hint="Duration, e.g. 3s"
              value={settings.settings.delay}
              onSave={(v) => saveSetting({ delay: v })}
            />
            <AutosaveField
              id="settings-arxiv-snapshot"
              type="text"
              label="arXiv snapshot path"
              fieldKey="arxivSnapshot"
              value={settings.settings.arxivSnapshot}
              onSave={(v) => saveSetting({ arxivSnapshot: v })}
            />
          </div>
        </fieldset>

        <fieldset className="flex flex-col gap-4 rounded-md border border-line p-4">
          <legend className="px-1 font-mono text-[13px] uppercase tracking-wide text-ink-2">Models</legend>
          <div className="grid grid-cols-2 gap-4">
            <AutosaveField
              id="settings-ollama-url"
              type="text"
              label="Ollama URL"
              fieldKey="ollamaUrl"
              value={settings.settings.ollamaUrl}
              onSave={(v) => saveSetting({ ollamaUrl: v })}
            />
            <AutosaveField
              id="settings-chat-model"
              type="text"
              label="Chat model"
              fieldKey="chatModel"
              value={settings.settings.chatModel}
              onSave={(v) => saveSetting({ chatModel: v })}
            />
            <AutosaveField
              id="settings-embed-model"
              type="text"
              label="Encoder model"
              fieldKey="embedModel"
              hint="Changing this needs a rebuild of existing projects."
              value={settings.settings.embedModel}
              onSave={(v) => saveSetting({ embedModel: v })}
            />
          </div>
        </fieldset>

        <fieldset className="flex flex-col gap-4 rounded-md border border-line p-4">
          <legend className="px-1 font-mono text-[13px] uppercase tracking-wide text-ink-2">API keys</legend>
          <div className="flex flex-col gap-4">
            <SecretField
              id="settings-openalex-key"
              label="OpenAlex API key"
              envVar={OPENALEX_ENV}
              fieldKey="openalexKey"
              hasKey={settings.settings.hasOpenalexKey}
              onSave={(v) => saveSetting({ openalexKey: v })}
            />
            <SecretField
              id="settings-semantic-scholar-key"
              label="Semantic Scholar API key"
              envVar={SEMANTIC_SCHOLAR_ENV}
              fieldKey="semanticScholarKey"
              hasKey={settings.settings.hasSemanticScholarKey}
              onSave={(v) => saveSetting({ semanticScholarKey: v })}
            />
            <SecretField
              id="settings-ollama-key"
              label="Ollama API key"
              envVar={OLLAMA_ENV}
              fieldKey="ollamaKey"
              hasKey={settings.settings.hasOllamaKey}
              onSave={(v) => saveSetting({ ollamaKey: v })}
            />
          </div>
        </fieldset>

        <fieldset className="flex flex-col gap-4 rounded-md border border-line p-4">
          <legend className="px-1 font-mono text-[13px] uppercase tracking-wide text-ink-2">Server</legend>
          <div className="grid grid-cols-2 gap-4">
            <AutosaveField
              id="settings-addr"
              type="text"
              label="Listen address"
              fieldKey="addr"
              hint="Applies next time the server starts."
              value={settings.settings.addr}
              onSave={(v) => saveSetting({ addr: v })}
            />
            <AutosaveField
              id="settings-project"
              type="text"
              label="Startup project"
              fieldKey="project"
              hint="Applies next time the server starts."
              value={settings.settings.project}
              onSave={(v) => saveSetting({ project: v })}
            />
            <AutosaveField
              id="settings-snapshot-nodes"
              type="number"
              label="Snapshot nodes"
              fieldKey="snapshotNodes"
              value={String(settings.settings.snapshotNodes)}
              onSave={(v) => saveSetting({ snapshotNodes: Number(v) })}
            />
            <TextField
              id="settings-data-dir"
              size="small"
              fullWidth
              label="Data directory"
              value={settings.dataDir}
              helperText="Set with --data at launch."
              slotProps={{ htmlInput: { readOnly: true } }}
            />
          </div>
        </fieldset>
      </div>
    </SectionShell>
  );
}
