"use client";
import Alert from "@mui/material/Alert";
import Typography from "@mui/material/Typography";
import { useMemo } from "react";
import ReactMarkdown, { type Components, type Options } from "react-markdown";
import rehypeRaw from "rehype-raw";
import remarkGfm from "remark-gfm";
import wikiLinkPlugin from "remark-wiki-link";

import { useAppStore } from "@/store";


const slug = (s: string) =>
  s
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");

const remarkPlugins: Options["remarkPlugins"] = [
  remarkGfm,
  [
    wikiLinkPlugin,
    {
      aliasDivider: "|",
      pageResolver: (name: string) => [slug(name)],
      hrefTemplate: (id: string) => `#node-${id}`,
      wikiLinkClassName: "wikilink",
      newClassName: "wikilink",
    },
  ],
];

const rehypePlugins: Options["rehypePlugins"] = [rehypeRaw];


export function WikiPane({ markdown }: { markdown?: string } = {}) {
  const wikiDoc = useAppStore((s) => s.wikiDoc);
  const wikiLoading = useAppStore((s) => s.wikiLoading);
  const wikiError = useAppStore((s) => s.wikiError);
  const nodes = useAppStore((s) => s.nodes);
  const selectNode = useAppStore((s) => s.selectNode);

  const nodeIds = useMemo(() => new Set(nodes.map((n) => n.id)), [nodes]);

  const components = useMemo<Components>(
    () => ({
      a: ({ className, href, children, node: _node, ...rest }) => {
        const isWikiLink = typeof className === "string" && className.includes("wikilink");
        if (isWikiLink) {
          const nodeId = (href ?? "").replace(/^#node-/, "");
          if (nodeIds.has(nodeId)) {
            return (
              <a
                {...rest}
                className={className}
                href={href}
                onClick={(e) => {
                  e.preventDefault();
                  selectNode(nodeId, nodeId);
                }}
              >
                {children}
              </a>
            );
          }
          return (
            <span
              className="wikilink cursor-default text-accent underline decoration-dotted underline-offset-2 opacity-70"
              title="Not in graph"
            >
              {children}
            </span>
          );
        }
        return (
          <a {...rest} className={className} href={href} target="_blank" rel="noreferrer">
            {children}
          </a>
        );
      },
    }),
    [nodeIds, selectNode],
  );

  if (wikiLoading) {
    return (
      <div className="flex flex-col gap-3" aria-hidden="true">
        <div className="h-4 w-2/3 animate-pulse rounded bg-bg-subtle" />
        <div className="h-4 w-full animate-pulse rounded bg-bg-subtle" />
        <div className="h-4 w-3/4 animate-pulse rounded bg-bg-subtle" />
      </div>
    );
  }

  if (wikiError) {
    return <Alert severity="error">{wikiError}</Alert>;
  }

  if (!wikiDoc) {
    return (
      <div className="flex h-full items-center justify-center">
        <Typography variant="caption" className="text-ink-2">
          Select a paper or graph node to open its wiki entry.
        </Typography>
      </div>
    );
  }

  return (
    <article className="wiki-prose">
      <ReactMarkdown
        remarkPlugins={remarkPlugins}
        rehypePlugins={rehypePlugins}
        components={components}
      >
        {markdown ?? wikiDoc.markdown}
      </ReactMarkdown>
    </article>
  );
}
