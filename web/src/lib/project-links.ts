/** URL of a project's Chat tab; with `docId`, it opens a new thread pinned to that paper. */
export function projectChatHref(projectId: string, docId?: string): string {
  const params = new URLSearchParams({ id: projectId, tab: "chat" });
  if (docId) params.set("pin", docId);
  return `/projects/detail?${params.toString()}`;
}
