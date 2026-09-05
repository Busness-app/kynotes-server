import { describe, expect, it } from "vitest";
import { parseBackupStatus } from "./backup";
const status = {key_id:"",paired:false,remote_url:"",local_directory:"",local_copies:[],interval_seconds:0,next_run:null,last_attempt:"",last_result:"",receipt:null,blob_count:2,blob_bytes:100,allow_private_recovery:false};
describe("backup status boundary", () => {
  it("represents an unconfigured instance without inventing coverage", () => { const result = parseBackupStatus(status); expect(result.keyID).toBe(""); expect(result.paired).toBe(false); expect(result.blobCount).toBe(2); expect(result.receipt).toBeNull(); });
  it("validates a deposit receipt", () => { const result = parseBackupStatus({...status,receipt:{capsule_id:"cap-test",digest:"digest",deposited_at:"2026-09-05T12:00:00Z"}}); expect(result.receipt?.id).toBe("cap-test"); });
  it("rejects malformed and unsafe counters", () => { for (const value of [null,{}, {...status,blob_count:-1},{...status,receipt:{}},{...status,local_copies:null}]) expect(() => parseBackupStatus(value)).toThrow(); });
});
