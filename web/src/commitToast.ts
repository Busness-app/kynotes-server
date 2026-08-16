export const COMMIT_TOAST_DURATION_MS = 15_000;

export function commitToastVisible(committedAt: number, now = Date.now()): boolean {
  return now - committedAt < COMMIT_TOAST_DURATION_MS;
}

export function commitToastLabel(committedAt: number, now = Date.now()): string {
  return `Last Committed ${Math.max(0, Math.floor((now - committedAt) / 1000))}s ago`;
}
