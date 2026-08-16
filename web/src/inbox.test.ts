import { describe, expect, it } from "vitest";
import { hasPersonalInbox, INBOX_KIND } from "./inbox";

describe("personal inbox", () => {
  it("requires a personal inbox and ignores team containers", () => {
    expect(hasPersonalInbox([{ kind: "workbook" }])).toBe(false);
    expect(hasPersonalInbox([{ kind: INBOX_KIND, teamId: "team-1" }])).toBe(false);
    expect(hasPersonalInbox([{ kind: INBOX_KIND }])).toBe(true);
  });
});
