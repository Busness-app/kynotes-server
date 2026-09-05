import { APIRequestError, loginParams, stepUp } from "./api";
import { deriveAuthSecret } from "./crypto";

/** Derives the login secret from a typed password, exactly as at login, and re-proves it for this session. */
export async function stepUpWithPassword(username: string, password: string): Promise<void> {
  const { loginSalt, iterations } = await loginParams(username);
  const authSecret = await deriveAuthSecret(password, loginSalt, iterations);
  await stepUp(authSecret);
}

export function isStepUpRequired(error: unknown): boolean {
  return error instanceof APIRequestError && error.code === "step_up_required";
}
