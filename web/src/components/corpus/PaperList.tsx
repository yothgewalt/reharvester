"use client";
import FormControlLabel from "@mui/material/FormControlLabel";
import List from "@mui/material/List";
import Switch from "@mui/material/Switch";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";
import { useMemo, useState } from "react";

import { useAppStore } from "@/store";

import { PaperListItem } from "./PaperListItem";


export function PaperList() {
  const nodes = useAppStore((s) => s.nodes);
  const selectedDocId = useAppStore((s) => s.selectedDocId);
  const selectNode = useAppStore((s) => s.selectNode);

  const [query, setQuery] = useState("");
  const [oaOnly, setOaOnly] = useState(false);

  const papers = useMemo(
    () =>
      nodes
        .filter((n) => n.data.kind === "paper")
        .map((n) => ({
          docId: n.data.docId,
          nodeId: n.id,
          title: n.label,
          year: n.data.year,
          openAccess: n.data.openAccess,
        }))
        .sort((a, b) => a.title.localeCompare(b.title)),
    [nodes],
  );

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return papers.filter(
      (p) => p.title.toLowerCase().includes(q) && (!oaOnly || p.openAccess),
    );
  }, [papers, query, oaOnly]);

  return (
    <div className="flex flex-col gap-3">
      <TextField
        size="small"
        label="Search"
        placeholder="Search corpus…"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        fullWidth
      />
      <FormControlLabel
        control={
          <Switch checked={oaOnly} onChange={(e) => setOaOnly(e.target.checked)} />
        }
        label={<Typography variant="caption">Open Access only</Typography>}
      />
      <Typography variant="caption" className="font-mono text-ink-2" aria-live="polite">
        {filtered.length} of {papers.length} works
      </Typography>
      <div className="max-h-[520px] overflow-y-auto pr-1">
        {filtered.length === 0 ? (
          <Typography variant="caption" className="text-ink-2">
            No matching works.
          </Typography>
        ) : (
          <List dense disablePadding>
            {filtered.map((p) => (
              <PaperListItem
                key={p.nodeId}
                title={p.title}
                year={p.year}
                openAccess={p.openAccess}
                selected={selectedDocId === p.docId}
                onSelect={() => selectNode(p.nodeId, p.docId, "external")}
              />
            ))}
          </List>
        )}
      </div>
    </div>
  );
}
