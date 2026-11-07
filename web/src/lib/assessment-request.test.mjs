import assert from "node:assert/strict";
import test from "node:test";
import { assessmentRequestIdentity, acceptAssessmentReceipt } from "./assessment-request.ts";

function storage() {
  const values = new Map();
  return {
    values,
    getItem: (key) => values.get(key) ?? null,
    setItem: (key, value) => values.set(key, value),
    removeItem: (key) => values.delete(key),
  };
}

test("ambiguous assessment retries reuse admission, while accepted starts can repeat", async () => {
  const session = storage();
  const input = { targets: ["fixture.example.test"], context: "a private test account" };
  const first = await assessmentRequestIdentity("purple-web", input, session);
  const retry = await assessmentRequestIdentity("purple-web", input, session);
  assert.equal(retry.id, first.id);
  assert.equal(session.values.get(first.key), first.id);
  assert.match(first.id, /^[0-9a-f]{8}-[0-9a-f-]{27,}$/i);
  assert.ok(!JSON.stringify([...session.values]).includes(input.context));
  assert.ok(!JSON.stringify([...session.values]).includes(input.targets[0]));
  const edited = await assessmentRequestIdentity("purple-web", { ...input, context: "different" }, session);
  assert.notEqual(edited.id, first.id);
  acceptAssessmentReceipt(first);
  assert.equal(session.values.has(first.key), false);
  const next = await assessmentRequestIdentity("purple-web", input, session);
  assert.notEqual(next.id, first.id);
  acceptAssessmentReceipt(next);
  acceptAssessmentReceipt(edited);
});

test("private-browsing storage failures retain an in-memory retry", async () => {
  const denied = {
    getItem() { throw Error("denied"); },
    setItem() { throw Error("denied"); },
    removeItem() { throw Error("denied"); },
  };
  const input = { targets: ["127.0.0.1"], context: "owned lab" };
  const first = await assessmentRequestIdentity("race", input, denied);
  const retry = await assessmentRequestIdentity("race", input, denied);
  assert.equal(retry.id, first.id);
  acceptAssessmentReceipt(first);
});
