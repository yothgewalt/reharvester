import Typography from "@mui/material/Typography";
import type { ReactNode } from "react";

type Tone = "white" | "faint" | "dark";

const toneClasses: Record<Tone, string> = {
  white: "bg-white",
  faint: "bg-bg-faint",
  dark: "bg-slab text-ink-inverse",
};

export interface SectionShellProps {
  id: string;
  eyebrow: string;
  title: string;
  description?: string;
  tone?: Tone;

  headerAside?: ReactNode;
  children: ReactNode;
}


export function SectionShell({
  id,
  eyebrow,
  title,
  description,
  tone = "white",
  headerAside,
  children,
}: SectionShellProps) {
  return (
    <section
      id={id}
      data-mui-color-scheme={tone === "dark" ? "dark" : undefined}
      className={`border-t border-line ${toneClasses[tone]}`}
    >
      <div className="mx-auto flex max-w-[1256px] flex-col gap-8 px-8 py-16">
        <header className="flex items-end justify-between gap-8">
          <div className="flex flex-col gap-2">
            <Typography
              variant="overline"
              className={tone === "dark" ? "text-ink-4" : "text-accent"}
            >
              {eyebrow}
            </Typography>
            <Typography variant="h4" component="h2">
              {title}
            </Typography>
            {description ? (
              <Typography
                variant="body1"
                className={tone === "dark" ? "text-ink-4" : "text-ink-2"}
              >
                {description}
              </Typography>
            ) : null}
          </div>
          {headerAside ? <div className="shrink-0">{headerAside}</div> : null}
        </header>
        {children}
      </div>
    </section>
  );
}
