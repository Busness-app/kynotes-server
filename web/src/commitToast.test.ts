import { describe, expect, it } from "vitest";
import { commitToastLabel, commitToastVisible, COMMIT_TOAST_DURATION_MS } from "./commitToast";

describe("commit toast", () => {
  it("shows elapsed seconds and expires at fifteen seconds", () => {
    const committedAt = 1_000;
    expect(commitToastLabel(committedAt, 4_250)).toBe("Last Committed 3s ago");
    expect(commitToastVisible(committedAt, committedAt + COMMIT_TOAST_DURATION_MS - 1)).toBe(true);
    expect(commitToastVisible(committedAt, committedAt + COMMIT_TOAST_DURATION_MS)).toBe(false);
  });
});
