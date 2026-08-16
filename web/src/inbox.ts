export const INBOX_KIND = "inbox";

export function hasPersonalInbox(containers: Array<{ kind: string; teamId?: string }>): boolean {
  return containers.some((container) => container.kind === INBOX_KIND && !container.teamId);
}
