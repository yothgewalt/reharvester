const WARN_MARKER = "__threeWarnFiltered";
const REJECTION_MARKER = "__templateForceRejectionHandled";

type MarkedWarn = typeof console.warn & { [WARN_MARKER]?: boolean };
type MarkedError = typeof console.error & { [REJECTION_MARKER]?: boolean };

if (typeof window !== "undefined") {
  if (!(console.warn as MarkedWarn)[WARN_MARKER]) {
    const originalWarn = console.warn.bind(console);
    const filteredWarn: MarkedWarn = (...args: Parameters<typeof console.warn>) => {
      if (typeof args[0] === "string") {
        if (
          args[0].includes("Clock: This module has been deprecated") ||
          args[0].includes("Attribute undefined is not a number for node")
        ) {
          return;
        }
      }
      originalWarn(...args);
    };
    filteredWarn[WARN_MARKER] = true;
    console.warn = filteredWarn;
  }

  if (!(console.error as MarkedError)[REJECTION_MARKER]) {
    const originalError = console.error.bind(console);
    const filteredError: MarkedError = (...args: Parameters<typeof console.error>) => {
      if (
        typeof args[0] === "string" &&
        args[0].includes("templateForce.tick")
      ) {
        return;
      }
      originalError(...args);
    };
    filteredError[REJECTION_MARKER] = true;
    console.error = filteredError;
  }

  window.addEventListener("unhandledrejection", (event) => {
    const reason = event.reason;
    if (
      reason instanceof TypeError &&
      typeof reason.stack === "string" &&
      reason.stack.includes("templateForce.tick")
    ) {
      event.preventDefault();
    }
  });
}

export {};
