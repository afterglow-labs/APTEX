type ReceiptStorage = Pick<Storage, "getItem" | "setItem" | "removeItem">;
const fallback = new Map<string, string>();

function sessionReceipts(): ReceiptStorage | undefined {
  try { return typeof window === "undefined" ? undefined : window.sessionStorage; } catch { return undefined; }
}

// Cache only an opaque request UUID and a payload digest, never targets, files,
// instructions, passwords, or bearer credentials. Ambiguous retries reuse the
// same admission after a page reload; accepted starts can intentionally repeat.
export async function assessmentRequestIdentity(workflow: string, input: object, storage = sessionReceipts()) {
  const bytes = new TextEncoder().encode(JSON.stringify({ workflow, input }));
  const digest = await crypto.subtle.digest("SHA-256", bytes);
  const hash = Array.from(new Uint8Array(digest), (value) => value.toString(16).padStart(2, "0")).join("");
  const key = `aptex.assessment-receipt.v1.${hash}`;
  let id = fallback.get(key);
  try { id = storage?.getItem(key) || id; } catch { /* private browsing may deny storage */ }
  id ||= crypto.randomUUID();
  fallback.set(key, id);
  try { storage?.setItem(key, id); } catch { /* in-memory retries remain idempotent */ }
  return { id, key, storage };
}

export function acceptAssessmentReceipt(receipt: Awaited<ReturnType<typeof assessmentRequestIdentity>>) {
  fallback.delete(receipt.key);
  try { receipt.storage?.removeItem(receipt.key); } catch { /* optional browser persistence */ }
}
