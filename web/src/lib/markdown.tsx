"use client";
import ReactMarkdown, { type Components, type Options } from "react-markdown";
import rehypeKatex from "rehype-katex";
import rehypeRaw from "rehype-raw";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";

export const proseRemarkPlugins: NonNullable<Options["remarkPlugins"]> = [remarkGfm, remarkMath];
export const proseRehypePlugins: NonNullable<Options["rehypePlugins"]> = [
  rehypeRaw,
  [rehypeKatex, { throwOnError: false, strict: "ignore" }],
];

export interface MarkdownProseProps {
  children: string;
  remarkPlugins?: Options["remarkPlugins"];
  components?: Components;
}

/**
 * GFM + KaTeX renderer shared by wiki entries and chat answers. The rehype
 * pipeline (raw HTML passthrough, KaTeX) is fixed; callers own their wrapping
 * element and can extend `remarkPlugins` (WikiPane adds wiki-link resolution)
 * or override `components` (e.g. to intercept link clicks).
 */
export function MarkdownProse({ children, remarkPlugins = proseRemarkPlugins, components }: MarkdownProseProps) {
  return (
    <ReactMarkdown remarkPlugins={remarkPlugins} rehypePlugins={proseRehypePlugins} components={components}>
      {children}
    </ReactMarkdown>
  );
}
