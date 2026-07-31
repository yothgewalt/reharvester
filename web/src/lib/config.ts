export const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8000";
export const WS_BASE = API_BASE.replace(/^http/, "ws");

export const USE_MOCKS = process.env.NEXT_PUBLIC_USE_MOCKS !== "false";
