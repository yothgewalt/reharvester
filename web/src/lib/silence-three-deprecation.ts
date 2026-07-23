






const MARKER = "__threeClockWarnFiltered";

type MarkedWarn = typeof console.warn & { [MARKER]?: boolean };

if (typeof window !== "undefined" && !(console.warn as MarkedWarn)[MARKER]) {
  const original = console.warn.bind(console);
  const filtered: MarkedWarn = (...args: Parameters<typeof console.warn>) => {
    if (
      typeof args[0] === "string" &&
      args[0].includes("Clock: This module has been deprecated")
    ) {
      return;
    }
    original(...args);
  };
  filtered[MARKER] = true;
  console.warn = filtered;
}

export {};
