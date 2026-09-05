import { APIRequestError, request } from "./api";

export type BackupStatus = {
  keyID: string; keyError: string; paired: boolean; remoteURL: string;
  localDirectory: string; localCopies: number; localError: string;
  intervalSeconds: number; nextRun: string | null; lastAttempt: string; lastResult: string;
  receipt: { id: string; digest: string; at: string } | null;
  blobCount: number; blobBytes: number; allowPrivate: boolean;
};
function record(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
function text(value: unknown): string { if (typeof value !== "string") throw new Error("Invalid backup response"); return value; }
function number(value: unknown): number { if (typeof value !== "number" || !Number.isSafeInteger(value) || value < 0) throw new Error("Invalid backup response"); return value; }
export function parseBackupStatus(value: unknown): BackupStatus {
  if (!record(value) || typeof value.paired !== "boolean" || typeof value.allow_private_recovery !== "boolean" || !Array.isArray(value.local_copies)) throw new Error("Invalid backup status");
  let receipt: BackupStatus["receipt"] = null;
  if (value.receipt !== null) {
    if (!record(value.receipt)) throw new Error("Invalid backup receipt");
    receipt = { id: text(value.receipt.capsule_id), digest: text(value.receipt.digest), at: text(value.receipt.deposited_at) };
  }
  return {
    keyID: text(value.key_id), keyError: text(value.key_error ?? ""), paired: value.paired,
    remoteURL: text(value.remote_url), localDirectory: text(value.local_directory), localCopies: value.local_copies.length,
    localError: text(value.local_error ?? ""), intervalSeconds: number(value.interval_seconds),
    nextRun: value.next_run === null ? null : text(value.next_run), lastAttempt: text(value.last_attempt), lastResult: text(value.last_result),
    receipt, blobCount: number(value.blob_count), blobBytes: number(value.blob_bytes), allowPrivate: value.allow_private_recovery,
  };
}
export async function backupStatus(): Promise<BackupStatus> { return parseBackupStatus(await request<unknown>("/api/v1/admin/backup/status")); }
export function backupAction(action: "pin-key" | "pair-remote" | "unpair" | "schedule", body: unknown = {}): Promise<unknown> {
  return request<unknown>(`/api/v1/admin/backup/${action}`, { method: "POST", body: JSON.stringify(body) });
}
export async function runBackup(): Promise<string> {
  const value = await request<unknown>("/api/v1/admin/backup/deposit", { method: "POST" });
  if (!record(value) || !record(value.result)) throw new Error("Invalid backup result");
  const local = typeof value.result.local_path === "string" && value.result.local_path !== "";
  const remote = record(value.result.receipt);
  const result = `Local copy: ${local ? "saved" : "not saved"}. KyRecovery: ${remote ? "deposited" : "not deposited"}.`;
  if (value.error_code || value.result.local_error) return `${result} A backup destination or receipt/audit write failed; check status before relying on this run.`;
  return result;
}
export async function runDrill(): Promise<string> {
  const value = await request<unknown>("/api/v1/admin/backup/drill", { method: "POST" });
  if (!record(value) || typeof value.passed !== "boolean") throw new Error("Invalid drill response");
  if (!value.passed) throw new Error("Restore drill failed. Check the restored database, secrets and inventory.");
  return "Synthetic restore drill passed. Ciphertext blobs and real custodian cards require separate recovery proof.";
}
export async function downloadCapsule(): Promise<void> {
  const response = await fetch("/api/v1/admin/backup/export-capsule", { credentials: "include" });
  if (!response.ok) {
    const value: unknown = await response.json();
    if (record(value) && record(value.error)) throw new APIRequestError(text(value.error.message), { error: { code: text(value.error.code) } });
    throw new Error("Capsule download failed");
  }
  const url = URL.createObjectURL(await response.blob());
  const link = document.createElement("a"); link.href = url; link.download = "KyNotes.kycap"; link.click();
  setTimeout(() => URL.revokeObjectURL(url), 60_000);
}
